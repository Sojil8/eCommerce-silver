package controllers

import (
	"errors"
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var roleAdmin string = "Admin"

type AdminDetails struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func ShowLoginPage(c *gin.Context) {
	pkg.Log.Info("Handling request to show admin login page")

	tokenString, err := c.Cookie("jwtTokensAdmin")
	if err == nil && tokenString != "" {
		pkg.Log.Debug("Found jwtTokensAdmin cookie", zap.String("token", tokenString[:10]+"...")) // Truncate for security
		claims := &middleware.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return middleware.SecretKey, nil
		})
		if err == nil && token.Valid {
			pkg.Log.Info("Valid admin token found, redirecting to dashboard", zap.String("email", claims.Email))
			c.Redirect(http.StatusSeeOther, "/admin/dashboard")
			return
		}
		pkg.Log.Warn("Invalid or expired admin token", zap.Error(err))
	} else {
		pkg.Log.Debug("No admin token found in cookie", zap.Error(err))
	}

	pkg.Log.Info("Rendering admin login page")
	c.HTML(http.StatusOK, "adminLogin.html", gin.H{
		"status": "ok",
	})
}

func AdminLogin(c *gin.Context) {
	pkg.Log.Info("Handling admin login request")

	var adminReq AdminDetails
	var adminCheck adminModels.Admin

	if err := c.ShouldBindJSON(&adminReq); err != nil {
		pkg.Log.Error("Failed to bind JSON data", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "binding data", "Failed to bind data", "")
		return
	}
	pkg.Log.Debug("Received login request", zap.String("email", adminReq.Email))

	if adminReq.Email == "" || adminReq.Password == "" {
		pkg.Log.Warn("Empty email or password provided")
		helper.ResponseWithErr(c, http.StatusUnauthorized, "Empty Email and Password", "Email and Password Required", "")
		return
	}

	check := database.DB.Where("email=? AND password=?", adminReq.Email, adminReq.Password).First(&adminCheck)
	if check.Error != nil {
		if errors.Is(check.Error, gorm.ErrRecordNotFound) {
			pkg.Log.Warn("Invalid email or password", zap.String("email", adminReq.Email))
			helper.ResponseWithErr(c, http.StatusUnauthorized, "Invalid Email or Password", "Invalid Email or Password", "")
			return
		}
		pkg.Log.Error("Database query failed", zap.String("email", adminReq.Email), zap.Error(check.Error))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "database query failed", "Database Querying failed", "")
		return
	}
	pkg.Log.Debug("Admin found in database", zap.Uint("admin_id", adminCheck.ID), zap.String("email", adminCheck.Email))

	token, err := middleware.GenerateToken(c, int(adminCheck.ID), adminCheck.Email, roleAdmin)
	if err != nil {
		pkg.Log.Error("Failed to generate JWT token", zap.String("email", adminCheck.Email), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Error with JWT Token", "JWT token is not creating", "")
		return
	}
	pkg.Log.Info("JWT token generated successfully", zap.String("email", adminCheck.Email), zap.String("token", token[:10]+"...")) // Truncate for security

	c.SetCookie("jwtTokensAdmin", token, 3600, "/", "", false, true)
	pkg.Log.Info("Admin login successful, cookie set", zap.String("email", adminCheck.Email))
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Login successful",
		"token":   token,
		"code":    http.StatusOK,
	})
}

func AdminLogout(c *gin.Context) {
	pkg.Log.Info("Handling admin logout request")

	c.SetCookie("jwtTokensAdmin", "", -1, "/", "", false, true)
	pkg.Log.Info("Admin logout successful, cookie cleared")
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Logout successful",
		"code":    http.StatusOK,
	})
}
