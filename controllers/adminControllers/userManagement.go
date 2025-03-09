package controllers

import (
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetUsers(c *gin.Context) {
	middleware.ClearCache()
	var users []userModels.User
	searchQuery := c.Query("search")

	dbQuery := database.DB.Unscoped().Order("id") // Simplified ordering

	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		dbQuery = dbQuery.Where("user_name ILIKE ? OR email ILIKE ? OR phone::text ILIKE ?",
			searchPattern, searchPattern, searchPattern)
	}

	// Enable GORM debug logging
	result := dbQuery.Debug().Find(&users)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": result.Error.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "customer.html", gin.H{
		"users":       users,
		"searchQuery": searchQuery,
	})
}

func BlockUser(c *gin.Context) {
	middleware.ClearCache()
	userID := c.Param("id")
	var user userModels.User

	if err := database.DB.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			helper.ResponseWithErr(c, http.StatusNotFound, "User Not Found", "Not Found User", "")
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error from database",
		})
	}

	result := database.DB.Model(&user).Update("is_blocked", true)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to block user",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "OK",
		"message": "Blocked user",
	})
}

func UnBlockUser(c *gin.Context) {
	middleware.ClearCache()
	userID := c.Param("id")
	var user userModels.User

	if err := database.DB.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			helper.ResponseWithErr(c, http.StatusNotFound, "User Not Found", "User Not Found", "")
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"database error": "Not Found In Database",
		})
		return
	}

	result := database.DB.Model(&user).Update("is_blocked", false)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to unblock user",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "OK",
		"message": "User Unblocked",
	})
}
