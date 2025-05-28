package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

var roleUser string = "User"


func ShowSignUp(c *gin.Context) {
	tokenString, err := c.Cookie("jwt_token")
	if err == nil && tokenString != "" {
		claims := &middleware.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			return middleware.SecretKey, nil
		})
		if err == nil && token.Valid {
			c.Redirect(http.StatusSeeOther, "/home")
			c.Abort()
			return
		}
	}
	c.HTML(http.StatusOK, "signup.html", nil)
}

var signupRequest struct {
	UserName        string `json:"username" form:"username" binding:"required"`
	Email           string `json:"email" form:"email" binding:"required,email"`
	Phone           string `json:"phone" form:"phone" binding:"required"`
	Password        string `json:"password" form:"password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirmpassword" form:"confirmpassword" binding:"required"`
}

func UserSignUp(c *gin.Context) {
	if err := c.ShouldBind(&signupRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "field": "all"})
		return
	}

	if database.DB.Where("email = ?", signupRequest.Email).First(&userModels.Users{}).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists", "field": "email"})
		return
	}

	if database.DB.Where("phone = ?", signupRequest.Phone).First(&userModels.Users{}).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Phone number already exists", "field": "phone"})
		return
	}

	if signupRequest.Password != signupRequest.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passwords do not match", "field": "confirmpassword"})
		return
	}

	hashedPassword, err := helper.HashPassword(signupRequest.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password", "field": "password"})
		return
	}

	otp, err := helper.GenerateAndStoreOTP(signupRequest.Email)
	if err != nil {
		log.Println("OTP generation/storage failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
		return
	}
	fmt.Println(otp)

	newUser := map[string]interface{}{
		"name":       signupRequest.UserName,
		"email":      signupRequest.Email,
		"phone":      signupRequest.Phone,
		"password":   hashedPassword,
		"otp":        otp,
		"created_at": time.Now().UTC(),
	}

	userData, err := json.Marshal(newUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize user data"})
		return
	}

	userKey := fmt.Sprintf("user:%s", signupRequest.Email)
	if err := database.RedisClient.Set(database.Ctx, userKey, userData, 15*time.Minute).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store user data in Redis"})
		return
	}

	if err := helper.SendOTP(signupRequest.Email, otp); err != nil {
		database.RedisClient.Del(database.Ctx, userKey)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send OTP", "field": "email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "OTP sent successfully",
		"redirect": fmt.Sprintf("/signup/otp?email=%s", signupRequest.Email),
	})
}

func ShowOTPPage(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.Redirect(http.StatusSeeOther, "/signup")
		return
	}
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
	if err := c.ShouldBind(&otpInput); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "field": "otp"})
		return
	}

	userKey := fmt.Sprintf("user:%s", otpInput.Email)
	exists, err := database.RedisClient.Exists(database.Ctx, userKey).Result()
	if err != nil {
		log.Println("Redis Exists Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check OTP existence"})
		return
	}
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "OTP expired or not found"})
		return
	}

	data, err := database.RedisClient.Get(database.Ctx, userKey).Result()
	if err != nil {
		log.Println("Redis Get Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve OTP data"})
		return
	}

	var storedData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &storedData); err != nil {
		log.Println("Unmarshal Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse OTP data"})
		return
	}

	storedOTP, ok := storedData["otp"].(string)
	if !ok || storedOTP != otpInput.OTP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OTP"})
		return
	}

	name, _ := storedData["name"].(string)
	email, _ := storedData["email"].(string)
	password, _ := storedData["password"].(string)
	phone, _ := storedData["phone"].(string)

	newUser := userModels.Users{
		UserName: name,
		Email:    email,
		Password: password,
		Phone:    phone,
	}

	if err := database.DB.Create(&newUser).Error; err != nil {
		log.Println("Database Create Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create user"})
		return
	}

	database.RedisClient.Del(database.Ctx, userKey)
	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "OTP verified successfully",
		"redirect": "/login",
	})
}

func ResendOTP(c *gin.Context) {
	var resendRequest struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&resendRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email"})
		return
	}

	userKey := fmt.Sprintf("user:%s", resendRequest.Email)
	exists, err := database.RedisClient.Exists(database.Ctx, userKey).Result()
	if err != nil {
		log.Println("Redis Exists Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user data"})
		return
	}
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No signup session found for this email"})
		return
	}

	otp, err := helper.GenerateAndStoreOTP(resendRequest.Email)
	if err != nil {
		log.Println("OTP generation/storage failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate new OTP"})
		return
	}
	fmt.Println(otp)
	data, err := database.RedisClient.Get(database.Ctx, userKey).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user data"})
		return
	}

	var userData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &userData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user data"})
		return
	}

	userData["otp"] = otp
	updatedData, err := json.Marshal(userData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize updated data"})
		return
	}

	if err := database.RedisClient.Set(database.Ctx, userKey, updatedData, 15*time.Minute).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update OTP in Redis"})
		return
	}

	if err := helper.SendOTP(resendRequest.Email, otp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resend OTP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "OTP resent successfully",
	})
}

func ShowLogin(c *gin.Context) {
	tokenString, err := c.Cookie("jwt_token")
	if err == nil && tokenString != "" {
		claims := &middleware.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			return middleware.SecretKey, nil
		})
		if err == nil && token.Valid {
			c.Redirect(http.StatusSeeOther, "/home")
			c.Abort()
			return
		}
	}
	c.HTML(http.StatusOK, "userlogin.html", gin.H{
		"title": "Login",
	})
}

var loginRequest struct {
	Email    string `json:"email" form:"email" binding:"required,email"`
	Password string `json:"password" form:"password" binding:"required,min=8"`
}


func LoginPostUser(c *gin.Context) {
	if err := c.ShouldBind(&loginRequest); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please provide a valid email and password", "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("email = ?", loginRequest.Email).First(&user).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User not found", "Email does not exist", "")
		return
	}

	if user.Is_blocked {
		helper.ResponseWithErr(c, http.StatusForbidden, "Account blocked", "Your account has been blocked", "")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginRequest.Password)); err != nil {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "Invalid credentials", "Incorrect password", "")
		return
	}

	token, err := middleware.GenerateToken(c, int(user.ID), user.Email, roleUser)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Token generation failed", "Error creating JWT", "")
		return
	}

	c.SetCookie("jwt_token", token, 24*60*60, "/", "", false, true)
	c.Redirect(http.StatusSeeOther, "/home")
}


func LogoutUser(c *gin.Context) {
	c.SetCookie("jwt_token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusSeeOther, "/login")
}
