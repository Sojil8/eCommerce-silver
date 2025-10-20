package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/services"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/Sojil8/eCommerce-silver/utils/storage"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func ShowForgotPassword(c *gin.Context) {
	pkg.Log.Info("Rendering forgot password page")
	c.HTML(http.StatusOK, "forgotPassword.html", gin.H{
		"title": "Forgot Password",
	})
}

var forgotPassRequest struct {
	Email string `json:"email" form:"email" binding:"required,email"`
}

func ForgotPasswordRequest(c *gin.Context) {
	pkg.Log.Info("Starting forgot password request process")

	if err := c.ShouldBindJSON(&forgotPassRequest); err != nil {
		pkg.Log.Error("Failed to bind JSON request",
			zap.String("email", forgotPassRequest.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please provide a valid email", "")
		return
	}

	pkg.Log.Debug("Forgot password request",
		zap.String("email", forgotPassRequest.Email))

	var user userModels.Users
	if err := database.DB.Where("email = ?", forgotPassRequest.Email).First(&user).Error; err != nil {
		pkg.Log.Warn("User not found",
			zap.String("email", forgotPassRequest.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Email does not exist", "")
		return
	}

	if user.IsBlocked {
		pkg.Log.Warn("User account is blocked",
			zap.String("email", forgotPassRequest.Email),
			zap.Uint("user_id", user.ID))
		helper.ResponseWithErr(c, http.StatusForbidden, "Account blocked", "Your account has been blocked", "")
		return
	}

	otp, err := helper.GenerateAndStoreOTP(forgotPassRequest.Email)
	if err != nil {
		pkg.Log.Error("Failed to generate or store OTP",
			zap.String("email", forgotPassRequest.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to generate OTP", "Internal error", "")
		return
	}

	pkg.Log.Debug("OTP generated successfully",
		zap.String("email", forgotPassRequest.Email))

	resetData := map[string]interface{}{
		"email": forgotPassRequest.Email,
		"otp":   otp,
	}
	data, err := json.Marshal(resetData)
	if err != nil {
		pkg.Log.Error("Failed to serialize reset data",
			zap.String("email", forgotPassRequest.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to serialize data", "Internal error", "")
		return
	}

	resetKey := fmt.Sprintf("reset:%s", forgotPassRequest.Email)
	if err := storage.RedisClient.Set(storage.Ctx, resetKey, data, 15*time.Minute).Err(); err != nil {
		pkg.Log.Error("Failed to store reset data in Redis",
			zap.String("email", forgotPassRequest.Email),
			zap.String("reset_key", resetKey),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to store reset data", "Internal error", "")
		return
	}

	if err := services.SendOTP(forgotPassRequest.Email, otp); err != nil {
		pkg.Log.Error("Failed to send OTP",
			zap.String("email", forgotPassRequest.Email),
			zap.Error(err))
		storage.RedisClient.Del(storage.Ctx, resetKey)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to send OTP", "Email sending failed", "")
		return
	}

	pkg.Log.Info("OTP sent successfully",
		zap.String("email", forgotPassRequest.Email),
		zap.String("reset_key", resetKey))

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "OTP sent successfully",
		"redirect": fmt.Sprintf("/forgot-password/otp?email=%s", forgotPassRequest.Email),
	})
}

func ShowResetOTPPage(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		pkg.Log.Warn("Email parameter missing for OTP page")
		c.Redirect(http.StatusSeeOther, "/forgot-password")
		return
	}
	pkg.Log.Info("Rendering OTP verification page",
		zap.String("email", email))
	c.HTML(http.StatusOK, "resetPassOTP.html", gin.H{
		"title": "Reset Password - OTP Verification",
		"email": email,
	})
}

var resetOTPInput struct {
	Email string `json:"email" form:"email" binding:"required,email"`
	OTP   string `json:"otp" form:"otp" binding:"required"`
}

func VerifyResetOTP(c *gin.Context) {
	pkg.Log.Info("Starting OTP verification process")

	if err := c.ShouldBind(&resetOTPInput); err != nil {
		pkg.Log.Error("Failed to bind OTP input",
			zap.String("email", resetOTPInput.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please provide a valid OTP", "")
		return
	}

	pkg.Log.Debug("OTP verification request",
		zap.String("email", resetOTPInput.Email))

	resetKey := fmt.Sprintf("reset:%s", resetOTPInput.Email)
	data, err := storage.RedisClient.Get(storage.Ctx, resetKey).Result()
	if err != nil {
		pkg.Log.Warn("OTP session expired or not found",
			zap.String("email", resetOTPInput.Email),
			zap.String("reset_key", resetKey),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "OTP expired or not found", "Session expired", "")
		return
	}

	var storedData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &storedData); err != nil {
		pkg.Log.Error("Failed to parse stored reset data",
			zap.String("email", resetOTPInput.Email),
			zap.String("reset_key", resetKey),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to parse data", "Internal error", "")
		return
	}

	storedOTP, ok := storedData["otp"].(string)
	if !ok || storedOTP != resetOTPInput.OTP {
		pkg.Log.Warn("Invalid OTP provided",
			zap.String("email", resetOTPInput.Email),
			zap.String("reset_key", resetKey))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid OTP", "OTP does not match", "")
		return
	}

	pkg.Log.Info("OTP verified successfully",
		zap.String("email", resetOTPInput.Email),
		zap.String("reset_key", resetKey))

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "OTP verified successfully",
		"redirect": fmt.Sprintf("/forgot-password/reset?email=%s", resetOTPInput.Email),
	})
}

func ShowResetPassword(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		pkg.Log.Warn("Email parameter missing for reset password page")
		c.Redirect(http.StatusSeeOther, "/forgot-password")
		return
	}
	pkg.Log.Info("Rendering reset password page",
		zap.String("email", email))
	c.HTML(http.StatusOK, "resetPassword.html", gin.H{
		"title": "Reset Password",
		"email": email,
	})
}

var resetPasswordRequest struct {
	Email           string `json:"email" form:"email" binding:"required,email"`
	Password        string `json:"password" form:"password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirmpassword" form:"confirmpassword" binding:"required"`
}

func ResetPassword(c *gin.Context) {
	pkg.Log.Info("Starting password reset process")

	if err := c.ShouldBind(&resetPasswordRequest); err != nil {
		pkg.Log.Error("Failed to bind password reset input",
			zap.String("email", resetPasswordRequest.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please provide valid password fields", "")
		return
	}

	if resetPasswordRequest.Password != resetPasswordRequest.ConfirmPassword {
		pkg.Log.Warn("Passwords do not match",
			zap.String("email", resetPasswordRequest.Email))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Passwords do not match", "Please ensure passwords match", "")
		return
	}

	resetKey := fmt.Sprintf("reset:%s", resetPasswordRequest.Email)
	if exists, _ := storage.RedisClient.Exists(storage.Ctx, resetKey).Result(); exists == 0 {
		pkg.Log.Warn("Reset session expired or invalid",
			zap.String("email", resetPasswordRequest.Email),
			zap.String("reset_key", resetKey))
		helper.ResponseWithErr(c, http.StatusForbidden, "Invalid session", "Reset session expired or invalid", "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("email = ?", resetPasswordRequest.Email).First(&user).Error; err != nil {
		pkg.Log.Warn("User not found",
			zap.String("email", resetPasswordRequest.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Email does not exist", "")
		return
	}

	hashedPassword, err := helper.HashPassword(resetPasswordRequest.Password)
	if err != nil {
		pkg.Log.Error("Failed to hash password",
			zap.String("email", resetPasswordRequest.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to hash password", "Internal error", "")
		return
	}

	user.Password = hashedPassword
	if err := database.DB.Save(&user).Error; err != nil {
		pkg.Log.Error("Failed to update user password",
			zap.String("email", resetPasswordRequest.Email),
			zap.Uint("user_id", user.ID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update password", "Database error", "")
		return
	}

	storage.RedisClient.Del(storage.Ctx, resetKey)
	pkg.Log.Info("Password reset successfully",
		zap.String("email", resetPasswordRequest.Email),
		zap.Uint("user_id", user.ID),
		zap.String("reset_key", resetKey))

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "Password reset successfully",
		"redirect": "/login",
	})
}