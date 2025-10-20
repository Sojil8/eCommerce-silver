package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"time"

	"github.com/Sojil8/eCommerce-silver/config"
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/services"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/Sojil8/eCommerce-silver/utils/storage"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var roleUser string = "User"

func ShowSignUp(c *gin.Context) {
	pkg.Log.Info("Starting signup page retrieval")

	tokenString, err := c.Cookie("jwt_token")
	if err == nil && tokenString != "" {
		claims := &middleware.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			return middleware.SecretKey, nil
		})
		if err == nil && token.Valid {
			pkg.Log.Debug("User already authenticated, redirecting to home",
				zap.String("email", claims.Email),
				zap.String("role", claims.Role))
			c.Redirect(http.StatusSeeOther, "/home")
			c.Abort()
			return
		}
		pkg.Log.Warn("Invalid or expired JWT token",
			zap.Error(err))
	}

	errorMessage := c.Query("error")
	if errorMessage == "" {
		errorMessage = c.Query("message")
	}

	pkg.Log.Debug("Rendering signup page",
		zap.String("error_message", errorMessage))

	c.HTML(http.StatusOK, "signup.html", gin.H{
		"error": errorMessage,
	})
}

var signupRequest struct {
	UserName        string `json:"username" form:"username" binding:"required"`
	Email           string `json:"email" form:"email" binding:"required,email"`
	Phone           string `json:"phone" form:"phone" binding:"required"`
	Password        string `json:"password" form:"password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirmpassword" form:"confirmpassword" binding:"required"`
}

func UserSignUp(c *gin.Context) {
	pkg.Log.Info("Starting user signup process")

	if err := c.ShouldBind(&signupRequest); err != nil {
		pkg.Log.Error("Failed to bind signup request",
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "field": "all"})
		return
	}

	pkg.Log.Debug("Processing signup request",
		zap.String("username", signupRequest.UserName),
		zap.String("email", signupRequest.Email),
		zap.String("phone_last4", signupRequest.Phone[len(signupRequest.Phone)-4:]))

	usernameRegex := regexp.MustCompile(`^[a-zA-Z]{2,}$`)
	if !usernameRegex.MatchString(signupRequest.UserName) {
		pkg.Log.Warn("Invalid username format",
			zap.String("username", signupRequest.UserName))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username must be at least 2 letters (no spaces allowed)"})
		return
	}

	_, err := mail.ParseAddress(signupRequest.Email)
	if err != nil {
		pkg.Log.Warn("Invalid email format",
			zap.String("email", signupRequest.Email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "not valid email",
		})
		return
	}

	if database.DB.Where("email = ?", signupRequest.Email).First(&userModels.Users{}).Error == nil {
		pkg.Log.Warn("Email already exists",
			zap.String("email", signupRequest.Email))
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists", "field": "email"})
		return
	}

	if database.DB.Where("phone = ?", signupRequest.Phone).First(&userModels.Users{}).Error == nil {
		pkg.Log.Warn("Phone number already exists",
			zap.String("phone_last4", signupRequest.Phone[len(signupRequest.Phone)-4:]))
		c.JSON(http.StatusConflict, gin.H{"error": "Phone number already exists", "field": "phone"})
		return
	}

	if signupRequest.Password != signupRequest.ConfirmPassword {
		pkg.Log.Warn("Passwords do not match")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passwords do not match", "field": "confirmpassword"})
		return
	}

	hashedPassword, err := helper.HashPassword(signupRequest.Password)
	if err != nil {
		pkg.Log.Error("Failed to hash password",
			zap.String("email", signupRequest.Email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password", "field": "password"})
		return
	}

	otp, err := helper.GenerateAndStoreOTP(signupRequest.Email)
	if err != nil {
		pkg.Log.Error("OTP generation/storage failed",
			zap.String("email", signupRequest.Email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
		return
	}

	referral := c.Query("ref")
	pkg.Log.Debug("Referral code provided",
		zap.String("referral_code", referral))

	newUser := map[string]interface{}{
		"name":       signupRequest.UserName,
		"email":      signupRequest.Email,
		"phone":      signupRequest.Phone,
		"password":   hashedPassword,
		"otp":        otp,
		"ref":        referral,
		"created_at": time.Now().UTC(),
	}

	userData, err := json.Marshal(newUser)
	if err != nil {
		pkg.Log.Error("Failed to serialize user data",
			zap.String("email", signupRequest.Email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize user data"})
		return
	}

	userKey := fmt.Sprintf("user:%s", signupRequest.Email)
	if err := storage.RedisClient.Set(storage.Ctx, userKey, userData, 15*time.Minute).Err(); err != nil {
		pkg.Log.Error("Failed to store user data in Redis",
			zap.String("email", signupRequest.Email),
			zap.String("user_key", userKey),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store user data in Redis"})
		return
	}

	if err := services.SendOTP(signupRequest.Email, otp); err != nil {
		pkg.Log.Error("Failed to send OTP",
			zap.String("email", signupRequest.Email),
			zap.Error(err))
		storage.RedisClient.Del(storage.Ctx, userKey)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send OTP", "field": "email"})
		return
	}

	pkg.Log.Info("OTP sent successfully",
		zap.String("email", signupRequest.Email),
		zap.String("user_key", userKey))

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "OTP sent successfully",
		"redirect": fmt.Sprintf("/signup/otp?email=%s", signupRequest.Email),
	})
}

func ShowOTPPage(c *gin.Context) {
	pkg.Log.Info("Starting OTP page retrieval")

	email := c.Query("email")
	if email == "" {
		pkg.Log.Warn("Missing email query parameter")
		c.Redirect(http.StatusSeeOther, "/signup")
		return
	}

	tokenString, err := c.Cookie("jwt_token")
	if err == nil && tokenString != "" {
		claims := &middleware.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			return middleware.SecretKey, nil
		})
		if err == nil && token.Valid {
			pkg.Log.Debug("User already authenticated, redirecting to home",
				zap.String("email", claims.Email),
				zap.String("role", claims.Role))
			c.Redirect(http.StatusSeeOther, "/home")
			return
		}
		pkg.Log.Warn("Invalid or expired JWT token",
			zap.Error(err))
	}

	userKey := fmt.Sprintf("user:%s", email)
	exists, err := storage.RedisClient.Exists(storage.Ctx, userKey).Result()
	if err != nil {
		pkg.Log.Error("Failed to check OTP session in Redis",
			zap.String("email", email),
			zap.String("user_key", userKey),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/signup?message=Session+expired.+Please+sign+up+again")
		return
	}
	if exists == 0 {
		pkg.Log.Warn("OTP session not found",
			zap.String("email", email),
			zap.String("user_key", userKey))
		c.Redirect(http.StatusSeeOther, "/signup?message=Session+expired.+Please+sign+up+again")
		return
	}

	verifiedKey := fmt.Sprintf("otp_verified:%s", email)
	verified, err := storage.RedisClient.Exists(storage.Ctx, verifiedKey).Result()
	if err != nil {
		pkg.Log.Warn("Failed to check OTP verified status",
			zap.String("email", email),
			zap.String("verified_key", verifiedKey),
			zap.Error(err))
	}
	if verified > 0 {
		pkg.Log.Info("OTP already verified, redirecting to login",
			zap.String("email", email),
			zap.String("verified_key", verifiedKey))
		c.Redirect(http.StatusSeeOther, "/login?message=Already+verified.+Please+login")
		return
	}

	pkg.Log.Debug("Rendering OTP page",
		zap.String("email", email))

	c.HTML(http.StatusOK, "otp.html", gin.H{
		"title": "OTP Verification",
		"email": email,
	})
}

var otpInput struct {
	Email string `json:"email" form:"email" binding:"required,email"`
	OTP   string `json:"otp" form:"otp" binding:"required,len=6"`
}

func VerifyOTP(c *gin.Context) {
	pkg.Log.Info("Starting OTP verification process")

	if err := c.ShouldBind(&otpInput); err != nil {
		pkg.Log.Error("Failed to bind OTP input",
			zap.String("email", otpInput.Email),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "field": "otp"})
		return
	}

	userKey := fmt.Sprintf("user:%s", otpInput.Email)
	exists, err := storage.RedisClient.Exists(storage.Ctx, userKey).Result()
	if err != nil {
		pkg.Log.Error("Failed to check OTP session in Redis",
			zap.String("email", otpInput.Email),
			zap.String("user_key", userKey),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check OTP existence"})
		return
	}
	if exists == 0 {
		pkg.Log.Warn("OTP session not found",
			zap.String("email", otpInput.Email),
			zap.String("user_key", userKey))
		c.JSON(http.StatusNotFound, gin.H{"error": "OTP expired or not found"})
		return
	}

	data, err := storage.RedisClient.Get(storage.Ctx, userKey).Result()
	if err != nil {
		pkg.Log.Error("Failed to retrieve OTP data from Redis",
			zap.String("email", otpInput.Email),
			zap.String("user_key", userKey),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve OTP data"})
		return
	}

	var storedData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &storedData); err != nil {
		pkg.Log.Error("Failed to unmarshal OTP data",
			zap.String("email", otpInput.Email),
			zap.String("user_key", userKey),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse OTP data"})
		return
	}

	storedOTP, ok := storedData["otp"].(string)
	if !ok || storedOTP != otpInput.OTP {
		pkg.Log.Warn("Invalid OTP provided",
			zap.String("email", otpInput.Email))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OTP"})
		return
	}

	name, _ := storedData["name"].(string)
	email, _ := storedData["email"].(string)
	password, _ := storedData["password"].(string)
	phone, _ := storedData["phone"].(string)
	ref, _ := storedData["ref"].(string)

	refralCode, err := helper.GenerateReferralCode()
	if err != nil {
		pkg.Log.Error("Failed to generate referral code",
			zap.String("email", email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Refral code Can't generate", "Refral code Can't generate", "")
		return
	}

	newUser := userModels.Users{
		UserName:      name,
		Email:         email,
		Password:      password,
		Phone:         phone,
		ReferralToken: refralCode,
	}

	if err := database.DB.Create(&newUser).Error; err != nil {
		pkg.Log.Error("Failed to create user in database",
			zap.String("email", email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create user"})
		return
	}

	newWallet := userModels.Wallet{
		UserID:  newUser.ID,
		Balance: 0,
	}

	if err := database.DB.Create(&newWallet).Error; err != nil {
		pkg.Log.Error("Failed to create user wallet",
			zap.Uint("user_id", newUser.ID),
			zap.String("email", email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create user wallet"})
		return
	}

	if ref != "" {
		if err := config.VerifiRefralCode(newUser.ID, ref); err != nil {
			pkg.Log.Error("Referral code verification failed",
				zap.Uint("user_id", newUser.ID),
				zap.String("referral_code", ref),
				zap.Error(err))
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Referral code verification failed", "Referral code verification failed", "")
			return
		}
		pkg.Log.Info("Referral code verified",
			zap.Uint("user_id", newUser.ID),
			zap.String("referral_code", ref))
	}

	storage.RedisClient.Del(storage.Ctx, userKey)
	pkg.Log.Debug("Deleted Redis OTP session",
		zap.String("email", email),
		zap.String("user_key", userKey))

	verifiedKey := fmt.Sprintf("otp_verified:%s", email)
	if err := storage.RedisClient.Set(storage.Ctx, verifiedKey, "true", 2*time.Minute).Err(); err != nil {
		pkg.Log.Warn("Failed to set OTP verified status in Redis",
			zap.String("email", email),
			zap.String("verified_key", verifiedKey),
			zap.Error(err))
	}

	pkg.Log.Info("OTP verified and user created successfully",
		zap.Uint("user_id", newUser.ID),
		zap.String("email", email),
		zap.String("username", name))

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "OTP verified successfully",
		"redirect": "/login",
	})
}

func ResendOTP(c *gin.Context) {
	pkg.Log.Info("Starting OTP resend process")

	var resendRequest struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&resendRequest); err != nil {
		pkg.Log.Error("Failed to bind resend OTP request",
			zap.String("email", resendRequest.Email),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email"})
		return
	}

	userKey := fmt.Sprintf("user:%s", resendRequest.Email)
	exists, err := storage.RedisClient.Exists(storage.Ctx, userKey).Result()
	if err != nil {
		pkg.Log.Error("Failed to check user session in Redis",
			zap.String("email", resendRequest.Email),
			zap.String("user_key", userKey),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user data"})
		return
	}
	if exists == 0 {
		pkg.Log.Warn("No signup session found",
			zap.String("email", resendRequest.Email),
			zap.String("user_key", userKey))
		c.JSON(http.StatusNotFound, gin.H{"error": "No signup session found for this email"})
		return
	}

	otp, err := helper.GenerateAndStoreOTP(resendRequest.Email)
	if err != nil {
		pkg.Log.Error("Failed to generate new OTP",
			zap.String("email", resendRequest.Email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate new OTP"})
		return
	}

	data, err := storage.RedisClient.Get(storage.Ctx, userKey).Result()
	if err != nil {
		pkg.Log.Error("Failed to retrieve user data from Redis",
			zap.String("email", resendRequest.Email),
			zap.String("user_key", userKey),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user data"})
		return
	}

	var userData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &userData); err != nil {
		pkg.Log.Error("Failed to unmarshal user data",
			zap.String("email", resendRequest.Email),
			zap.String("user_key", userKey),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user data"})
		return
	}

	userData["otp"] = otp
	updatedData, err := json.Marshal(userData)
	if err != nil {
		pkg.Log.Error("Failed to serialize updated user data",
			zap.String("email", resendRequest.Email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize updated data"})
		return
	}

	if err := storage.RedisClient.Set(storage.Ctx, userKey, updatedData, 15*time.Minute).Err(); err != nil {
		pkg.Log.Error("Failed to update OTP in Redis",
			zap.String("email", resendRequest.Email),
			zap.String("user_key", userKey),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update OTP in Redis"})
		return
	}

	if err := services.SendOTP(resendRequest.Email, otp); err != nil {
		pkg.Log.Error("Failed to resend OTP",
			zap.String("email", resendRequest.Email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resend OTP"})
		return
	}

	pkg.Log.Info("OTP resent successfully",
		zap.String("email", resendRequest.Email),
		zap.String("user_key", userKey))

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "OTP resent successfully",
	})
}

func ShowLogin(c *gin.Context) {
	pkg.Log.Info("Starting login page retrieval")

	tokenString, err := c.Cookie("jwt_token")
	if err == nil && tokenString != "" {
		claims := &middleware.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			return middleware.SecretKey, nil
		})
		if err == nil && token.Valid {
			pkg.Log.Debug("User already authenticated, redirecting to home",
				zap.String("email", claims.Email),
				zap.String("role", claims.Role))
			c.Redirect(http.StatusSeeOther, "/home")
			c.Abort()
			return
		}
		pkg.Log.Warn("Invalid or expired JWT token",
			zap.Error(err))
	}

	pkg.Log.Debug("Rendering login page")

	c.HTML(http.StatusOK, "userlogin.html", gin.H{
		"title": "Login",
	})
}

var loginRequest struct {
	Email    string `json:"email" form:"email" binding:"required,email"`
	Password string `json:"password" form:"password" binding:"required,min=8"`
}

func LoginPostUser(c *gin.Context) {
	pkg.Log.Info("Starting user login process")

	if err := c.ShouldBind(&loginRequest); err != nil {
		pkg.Log.Error("Failed to bind login request",
			zap.String("email", loginRequest.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please provide a valid email and password", "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("email = ?", loginRequest.Email).First(&user).Error; err != nil {
		pkg.Log.Warn("User not found",
			zap.String("email", loginRequest.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User not found", "Email does not exist", "")
		return
	}

	if user.IsBlocked {
		pkg.Log.Warn("User account blocked",
			zap.Uint("user_id", user.ID),
			zap.String("email", loginRequest.Email))
		helper.ResponseWithErr(c, http.StatusForbidden, "Account blocked", "Your account has been blocked", "")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginRequest.Password)); err != nil {
		pkg.Log.Warn("Incorrect password",
			zap.Uint("user_id", user.ID),
			zap.String("email", loginRequest.Email))
		helper.ResponseWithErr(c, http.StatusUnauthorized, "Invalid credentials", "Incorrect password", "")
		return
	}

	token, err := middleware.GenerateToken(c, int(user.ID), user.Email, roleUser)
	if err != nil {
		pkg.Log.Error("Failed to generate JWT token",
			zap.Uint("user_id", user.ID),
			zap.String("email", loginRequest.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Token generation failed", "Error creating JWT", "")
		return
	}

	pkg.Log.Info("User logged in successfully",
		zap.Uint("user_id", user.ID),
		zap.String("email", loginRequest.Email),
		zap.String("role", roleUser))

	c.SetCookie("jwt_token", token, 24*60*60, "/", "", false, true)
	c.Redirect(http.StatusSeeOther, "/home")
}

func LogoutUser(c *gin.Context) {
	pkg.Log.Info("Starting user logout process")

	userID, exists := c.Get("id")
	if exists {
		userIDUint, ok := userID.(uint)
		if ok {
			pkg.Log.Info("User logged out successfully",
				zap.Uint("user_id", userIDUint))
		} else {
			pkg.Log.Warn("Invalid user ID type during logout",
				zap.Any("user_id", userID))
		}
	} else {
		pkg.Log.Debug("No user ID in context for logout")
	}

	c.SetCookie("jwt_token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusSeeOther, "/login")
}
