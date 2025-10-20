package controllers

import (
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func GetUsers(c *gin.Context) {
	pkg.Log.Info("Handling request to get users")

	searchQuery := c.Query("search")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	pkg.Log.Debug("Received query parameters",
		zap.String("search", searchQuery),
		zap.String("page", pageStr),
		zap.String("limit", limitStr))

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		pkg.Log.Warn("Invalid page number, defaulting to 1", zap.String("page", pageStr), zap.Error(err))
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		pkg.Log.Warn("Invalid limit, defaulting to 10", zap.String("limit", limitStr), zap.Error(err))
		limit = 10
	}
	pkg.Log.Debug("Parsed pagination parameters", zap.Int("page", page), zap.Int("limit", limit))

	offset := (page - 1) * limit
	dbQuery := database.DB.Unscoped().Order("id")

	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		dbQuery = dbQuery.Where("user_name ILIKE ?", searchPattern)
		pkg.Log.Debug("Applied search filter", zap.String("search_pattern", searchPattern))
	}

	var totalUsers int64
	if err := dbQuery.Model(&userModels.Users{}).Count(&totalUsers).Error; err != nil {
		pkg.Log.Error("Failed to count users", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to count users", err.Error(), "")
		return
	}
	pkg.Log.Debug("Total users counted", zap.Int64("total_users", totalUsers))

	var users []userModels.Users
	if err := dbQuery.Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		pkg.Log.Error("Failed to fetch users", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch users", err.Error(), "")
		return
	}
	pkg.Log.Debug("Users retrieved", zap.Int("user_count", len(users)))

	totalPages := int((totalUsers + int64(limit) - 1) / int64(limit))
	pkg.Log.Debug("Calculated pagination", zap.Int("total_pages", totalPages))

	pkg.Log.Info("Rendering customer.html",
		zap.Int("user_count", len(users)),
		zap.Int("current_page", page),
		zap.Int("total_pages", totalPages),
		zap.String("search_query", searchQuery))
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

func BlockUser(c *gin.Context) {
	pkg.Log.Info("Handling request to block user")

	userID := c.Param("id")
	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		pkg.Log.Warn("Invalid user ID", zap.String("id", userID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid user ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed user ID", zap.Uint64("id", id))

	var user userModels.Users
	if err := database.DB.First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			pkg.Log.Warn("User not found", zap.Uint64("id", id))
			helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "User not found", "")
			return
		}
		pkg.Log.Error("Failed to fetch user", zap.Uint64("id", id), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch user", err.Error(), "")
		return
	}
	pkg.Log.Debug("User retrieved", zap.Uint("id", user.ID), zap.String("user_name", user.UserName))

	if user.IsBlocked {
		pkg.Log.Warn("User already blocked", zap.Uint("id", user.ID), zap.String("user_name", user.UserName))
		helper.ResponseWithErr(c, http.StatusConflict, "User already blocked", "User is already blocked", "")
		return
	}

	if err := database.DB.Model(&user).Update("is_blocked", true).Error; err != nil {
		pkg.Log.Error("Failed to block user", zap.Uint("id", user.ID), zap.String("user_name", user.UserName), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to block user", err.Error(), "")
		return
	}

	pkg.Log.Info("User blocked successfully", zap.Uint("id", user.ID), zap.String("user_name", user.UserName))
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "User blocked successfully",
	})
}

func UnBlockUser(c *gin.Context) {
	pkg.Log.Info("Handling request to unblock user")

	userID := c.Param("id")
	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		pkg.Log.Warn("Invalid user ID", zap.String("id", userID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid user ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed user ID", zap.Uint64("id", id))

	var user userModels.Users
	if err := database.DB.First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			pkg.Log.Warn("User not found", zap.Uint64("id", id))
			helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "User not found", "")
			return
		}
		pkg.Log.Error("Failed to fetch user", zap.Uint64("id", id), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch user", err.Error(), "")
		return
	}
	pkg.Log.Debug("User retrieved", zap.Uint("id", user.ID), zap.String("user_name", user.UserName))

	if !user.IsBlocked {
		pkg.Log.Warn("User already unblocked", zap.Uint("id", user.ID), zap.String("user_name", user.UserName))
		helper.ResponseWithErr(c, http.StatusConflict, "User already unblocked", "User is already unblocked", "")
		return
	}

	if err := database.DB.Model(&user).Update("is_blocked", false).Error; err != nil {
		pkg.Log.Error("Failed to unblock user", zap.Uint("id", user.ID), zap.String("user_name", user.UserName), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to unblock user", err.Error(), "")
		return
	}

	pkg.Log.Info("User unblocked successfully", zap.Uint("id", user.ID), zap.String("user_name", user.UserName))
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "User unblocked successfully",
	})
}