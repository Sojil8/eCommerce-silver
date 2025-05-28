package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
)

func ShowForgotPassword(c *gin.Context) {
	c.HTML(http.StatusOK, "forgotPassword.html", gin.H{
		"title": "Forgot Password",
	})
}

var forgotPassRequest struct {
	Email string `json:"email" form:"email" binding:"required,email"`
}

func ForgotPasswordRequest(c *gin.Context) {
	if err := c.ShouldBindJSON(&forgotPassRequest); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please provide a valid email", "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("email = ?", forgotPassRequest.Email).First(&user).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Email does not exist", "")
		return
	}

	if user.Is_blocked {
		helper.ResponseWithErr(c, http.StatusForbidden, "Account blocked", "Your account has been blocked", "")
		return
	}

	otp, err := helper.GenerateAndStoreOTP(forgotPassRequest.Email)
	fmt.Println("Generated OTP:", otp)
	if err != nil {
		log.Println("OTP generation/storage failed:", err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to generate OTP", "Internal error", "")
		return
	}

	resetData := map[string]interface{}{
		"email": forgotPassRequest.Email,
		"otp":   otp,
	}
	data, err := json.Marshal(resetData)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to serialize data", "Internal error", "")
		return
	}

	resetKey := fmt.Sprintf("reset:%s", forgotPassRequest.Email)
	if err := database.RedisClient.Set(database.Ctx, resetKey, data, 15*time.Minute).Err(); err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to store reset data", "Internal error", "")
		return
	}

	if err := helper.SendOTP(forgotPassRequest.Email, otp); err != nil {
		fmt.Println("Failed to send OTP:", otp)
		database.RedisClient.Del(database.Ctx, resetKey)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to send OTP", "Email sending failed", "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "OTP sent successfully",
		"redirect": fmt.Sprintf("/forgot-password/otp?email=%s", forgotPassRequest.Email),
	})
}

func ShowResetOTPPage(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.Redirect(http.StatusSeeOther, "/forgot-password")
		return
	}
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
	if err := c.ShouldBind(&resetOTPInput); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please provide a valid OTP", "")
		return
	}

	resetKey := fmt.Sprintf("reset:%s", resetOTPInput.Email)
	data, err := database.RedisClient.Get(database.Ctx, resetKey).Result()
	if err != nil {
		log.Println("Redis Get Error:", err)
		helper.ResponseWithErr(c, http.StatusNotFound, "OTP expired or not found", "Session expired", "")
		return
	}

	var storedData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &storedData); err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to parse data", "Internal error", "")
		return
	}

	storedOTP, ok := storedData["otp"].(string)
	if !ok || storedOTP != resetOTPInput.OTP {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid OTP", "OTP does not match", "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "OTP verified successfully",
		"redirect": fmt.Sprintf("/forgot-password/reset?email=%s", resetOTPInput.Email),
	})
}

func ShowResetPassword(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.Redirect(http.StatusSeeOther, "/forgot-password")
		return
	}
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
	if err := c.ShouldBind(&resetPasswordRequest); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please provide valid password fields", "")
		return
	}

	if resetPasswordRequest.Password != resetPasswordRequest.ConfirmPassword {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Passwords do not match", "Please ensure passwords match", "")
		return
	}

	resetKey := fmt.Sprintf("reset:%s", resetPasswordRequest.Email)
	if exists, _ := database.RedisClient.Exists(database.Ctx, resetKey).Result(); exists == 0 {
		helper.ResponseWithErr(c, http.StatusForbidden, "Invalid session", "Reset session expired or invalid", "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("email = ?", resetPasswordRequest.Email).First(&user).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Email does not exist", "")
		return
	}

	hashedPassword, err := helper.HashPassword(resetPasswordRequest.Password)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to hash password", "Internal error", "")
		return
	}

	user.Password = hashedPassword
	if err := database.DB.Save(&user).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update password", "Database error", "")
		return
	}

	database.RedisClient.Del(database.Ctx, resetKey)
	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "Password reset successfully",
		"redirect": "/login",
	})
}
