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

var roleUser string = "user"

func ShowSignUp(c *gin.Context) {
	tokenString, err := c.Cookie("jwtTokensUser")
	if err == nil && tokenString != "" {
		claims := &middleware.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(c *jwt.Token) (interface{}, error) {
			return middleware.SecretKey, nil
		})
		if err == nil && token.Valid {
			c.Redirect(http.StatusSeeOther, "/")
			c.Abort()
			return
		}
	}
	c.HTML(http.StatusOK, "signup.html", nil)
}

func UserSignUp(c *gin.Context) {
	var request struct {
		UserName        string `json:"username" form:"username" binding:"required"`
		Email           string `json:"email" form:"email" binding:"required"`
		Phone           string `json:"phone" form:"phone" binding:"required"`
		Password        string `json:"password" form:"password" binding:"required"`
		ConfirmPassword string `json:"confirmpassword" form:"confirmpassword" binding:"required"`
	}

	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "field": "all"})
		return
	}

	if database.DB.Where("email = ?", request.Email).First(&userModels.User{}).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists", "field": "email"})
		return
	}

	if database.DB.Where("phone = ?", request.Phone).First(&userModels.User{}).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Phone number already exists", "field": "phone"})
		return
	}

	if request.Password != request.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passwords do not match", "field": "confirmpassword"})
		return
	}

	hashedPassword, err := helper.HashPassword(request.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error", "field": "password"})
		return
	}

	otp := helper.GenerateOTP()
	fmt.Println("Generated OTP:", otp)
	if otp == "" {
		log.Println("OTP generation failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	newUser := map[string]interface{}{
		"name":       request.UserName,
		"email":      request.Email,
		"phone":      request.Phone, // Keep as string for now, convert later
		"password":   hashedPassword,
		"otp":        otp,
		"created_at": time.Now().UTC(),
	}

	userData, err := json.Marshal(newUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	userKey := fmt.Sprintf("user:%s", request.Email)
	if err := database.RedisClient.Set(database.Ctx, userKey, userData, 15*time.Minute).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if err := helper.SendOTP(request.Email, otp); err != nil {
		database.RedisClient.Del(database.Ctx, userKey)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send OTP", "field": "email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "OTP sent successfully",
		"redirect": fmt.Sprintf("/user/signup/otp?email=%s", request.Email),
	})
}

func ShowOTPPage(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.Redirect(http.StatusSeeOther, "/user/signup")
		return
	}
	c.HTML(http.StatusOK, "otp.html", gin.H{
		"title": "OTP Verification",
		"email": email,
	})
}

func VerifyOTP(c *gin.Context) {
	var input struct {
		Email string `json:"email" form:"email" binding:"required"`
		OTP   string `json:"otp" form:"otp" binding:"required"`
	}
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	userKey := fmt.Sprintf("user:%s", input.Email)

	exist, err := database.RedisClient.Exists(database.Ctx, userKey).Result()
	if err != nil {
		log.Println("Redis Exists Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check Redis key"})
		return
	}
	if exist == 0 {
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
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid OTP format"})
		return
	}

	if storedOTP != input.OTP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OTP"})
		return
	}

	name, ok := storedData["name"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data format for name"})
		return
	}

	email, ok := storedData["email"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data format for email"})
		return
	}

	password, ok := storedData["password"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data format for password"})
		return
	}

	phone, ok := storedData["phone"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data format for phone"})
		return
	}

	newUser := userModels.User{
		UserName: name,
		Email:    email,
		Password: password,
		Phone:    phone, // Directly use the string, no conversion needed
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
		"redirect": "/user/login",
	})
}

// ShowLogin and LoginPostUser remain unchanged
func ShowLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "userlogin.html", gin.H{
		"title": "Login",
	})
}

func LoginPostUser(c *gin.Context) {
	fmt.Println("----------------user login------------------")
	var request struct {
		Email    string `json:"email" form:"email" binding:"required"`
		Password string `json:"password" form:"password" binding:"required"`
	}

	if err := c.ShouldBind(&request); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Error in binding data", "Error in binding data", "")
		return
	}

	var user userModels.User
	if err := database.DB.Where("email=?", request.Email).First(&user).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User not Found", "User not Found", "")
		return
	}

	if user.Is_blocked {
		helper.ResponseWithErr(c, http.StatusForbidden, "Account blocked", "Your Account Has Been blocked", "")
		return
	}

	// fmt.Println("Request password:",request.Password,"User password:",user.Password)s

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password)); err != nil {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "Invalid Credential", "Incorrect Password", "")
		return
	}

	token, err := middleware.GenerateToken(c, user.ID, user.UserName, roleUser)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Token generation failed", "Error In jwt Creating", "")
		return
	}

	c.SetCookie("jwt_token", token, 24*60*60, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"status":  "OK",
		"message": "Login successful",
	})
}
