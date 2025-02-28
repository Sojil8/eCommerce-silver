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

//used ======database.RedisClient

func UserSignUp(c *gin.Context) {
	fmt.Println("---------------------------user signup--------------------------")
	var request struct {
		UserName        string `json:"username" form:"username"`
		Email           string `json:"email" form:"email"`
		Password        string `json:"password" form:"password"`
		ConfirmPassword string `json:"confirmpassword" form:"confirmpassword"`
	}
	if err := c.ShouldBind(&request); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "invalid input", "invalid input", "")
		return
	}
	fmt.Println(request)
	var exitsUser userModels.User
	if err := database.DB.Where("email = ?", request.Email).First(&exitsUser).Error; err == nil {
		helper.ResponseWithErr(c, http.StatusConflict, "Email already Exits", "Email already Exits", "")
		return
	}

	if request.Password != request.ConfirmPassword {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Passwords do not match", "Passwords do not match", "")
		return
	}

	hashedPassword, err := helper.HashPassword(request.Password)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Password Hash error", "Error in password hashing", "")
		return
	}

	otp := helper.GenerateOTP()

	fmt.Println(otp)
	if otp == "" {
		log.Fatal("OTP is not creating")
	}

	newUser := map[string]interface{}{
		"name":       request.UserName,
		"email":      request.Email,
		"password":   hashedPassword,
		"otp":        otp,
		"created_at": time.Now().UTC(),
	}
	userData, err := json.Marshal(newUser)
	if err != nil {
		log.Fatal("error to get marshal")
		return
	}

	userKey := fmt.Sprintf("user%s", request.Email)

	if err := database.RedisClient.Set(database.Ctx, userKey, userData, 15*time.Minute).Err(); err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Redis error", "Failed to store user data in Redis", "")
		log.Fatal("error in redis storing data")
		return
	}

	if err := helper.SendOTP(request.Email, otp); err != nil {
		database.RedisClient.Del(database.Ctx, request.Email)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Faild to send otp", "Faild to send otp", "")
		return
	}

	// In UserSignUp, replace the redirect:
	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/user/signup/otp?email=%s", request.Email))

}

func ShowOTPPage(c *gin.Context) {
	email := c.Query("email") // Get email from query param or another method
	if email == "" {
		// Fallback: redirect back to signup if email isn’t provided
		c.Redirect(http.StatusSeeOther, "/user/signup")
		return
	}
	c.HTML(http.StatusOK, "otp.html", gin.H{
		"title": "OTP Verification",
		"email": email, // Pass email to the template
	})
}

func VerifyOTP(c *gin.Context) {
	var input struct {
		Email string `json:"email" form:"email"`
		OTP   string `json:"otp"  form:"otp"`
	}
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userKey := fmt.Sprintf("user%s", input.Email)

	exist, err := database.RedisClient.Exists(database.Ctx, userKey).Result()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check Redis key"})
		return
	}

	if exist == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Key not found in Redis"})
		return
	}

	data, err := database.RedisClient.Get(database.Ctx, userKey).Result()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Redis data is not passing"})
		return
	}

	var storedData map[string]interface{}
	json.Unmarshal([]byte(data), &storedData)

	storedOTP := fmt.Sprintf("%v", storedData["otp"])
	if storedOTP != input.OTP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OTP"})
		return
	}

	fmt.Printf("Stored OTP (type: %T, value: %v)\n", storedData["otp"], storedData["otp"])
	fmt.Printf("Input OTP (type: %T, value: %v)\n", input.OTP, input.OTP)

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

	newUser := userModels.User{
		UserName: name,
		Email:    email,
		Password: password,
	}

	if err := database.DB.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create user"})
		return
	}

	database.RedisClient.Del(c, userKey)

	c.JSON(http.StatusOK, gin.H{
		"stauts": "ok",
	})
}
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
	
	fmt.Println("Request password:",request.Password,"User password:",user.Password)

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
