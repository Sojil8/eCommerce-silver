package controllers

import (
	"errors"
	"fmt"

	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"gorm.io/gorm"
)

var roleAdmin string = "Admin"

type AdminDetails struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func ShowLoginPage(c *gin.Context) {
    tokenString, err := c.Cookie("jwtTokensAdmin")
    if err == nil && tokenString != "" {
        claims := &middleware.Claims{}
        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
            return middleware.SecretKey, nil
        })
        if err == nil && token.Valid {
            c.Redirect(http.StatusSeeOther, "/admin/user-management")
            return
        }
    }
    c.HTML(http.StatusOK, "adminLogin.html", gin.H{
        "status": "ok",
    })
}


func AdminLogin(c *gin.Context) {
	var adminReq AdminDetails
	var adminCheck adminModels.Admin

	if err := c.ShouldBindJSON(&adminReq); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "binding data", "Faild to bind data", "")
		return
	}

	if adminReq.Email == "" || adminReq.Password == "" {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "Empty Email and Passwod", "Email and Passwod Required", "")
		return
	}

	check := database.DB.Where("email=? AND password=?", adminReq.Email, adminReq.Password).First(&adminCheck)
	if check.Error != nil {
		if errors.Is(check.Error, gorm.ErrRecordNotFound) {
			helper.ResponseWithErr(c, http.StatusUnauthorized, "Invalid Email or Password", "Invalid Email or Password", "")
			return
		} else {
			helper.ResponseWithErr(c, http.StatusInternalServerError, "database query faild", "Database Querying faild", "")
		}
	}

	token, err := middleware.GenerateToken(c, int(adminCheck.ID), adminCheck.Email, roleAdmin)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Error with JWT Token", "JWT token is not creating", "")
		return
	}
	c.SetCookie("jwtTokensAdmin", token, 3600, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Login successful",
		"token":   token,
		"code":    http.StatusOK,
	})
	fmt.Println("-----------------admin login succesfull--------------------")

}

func AdminLogout(c *gin.Context) {
	c.SetCookie("jwtTokensAdmin", "", -1, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Logout successful",
		"code":    http.StatusOK,
	})
}
