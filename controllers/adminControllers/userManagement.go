package controllers

import (
	"net/http"
	"strconv"

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
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10
	}

	offset := (page - 1) * limit
	dbQuery := database.DB.Unscoped().Order("id")

	// Search only by user_name
	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		dbQuery = dbQuery.Where("user_name ILIKE ?", searchPattern) // Only search user_name
	}

	var totalUsers int64
	if err := dbQuery.Model(&userModels.User{}).Count(&totalUsers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := dbQuery.Limit(limit).Offset(offset).Find(&users)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	totalPages := int((totalUsers + int64(limit) - 1) / int64(limit))

	// Pass all necessary pagination data to the template
	c.HTML(http.StatusOK, "customer.html", gin.H{
		"users":       users,
		"searchQuery": searchQuery,
		"page":        page,
		"limit":       limit,
		"totalUsers":  totalUsers,
		"totalPages":  totalPages,
		"prevPage":    page - 1,
		"nextPage":    page + 1,
		"hasPrev":     page > 1,
		"hasNext":     page < totalPages,
	})
}

// BlockUser and UnBlockUser remain unchanged
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