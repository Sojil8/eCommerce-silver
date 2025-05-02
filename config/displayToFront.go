package config

import (
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupRoutes(r *gin.Engine, db *gorm.DB) {
	// Apply middleware to routes that need user data
	authenticated := r.Group("/", middleware.Authenticate("jwt_token", "User", "/login"))
	authenticated.GET("/home", func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(500, gin.H{"error": "User not found in context"})
			return
		}

		userData := user.(userModels.Users)

		// Fetch Wishlist and Cart counts (assuming you have models for these)
		var wishlistCount, cartCount int64
		if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).
			Error; err != nil {
			wishlistCount = 0
		}

		if err := database.DB.Model(&userModels.Cart{}).Where("user_id = ?", userData.ID).Count(&cartCount).
			Error; err != nil {
			cartCount = 0
		}

		c.HTML(200, "home.html", gin.H{
			"UserName":      userData.UserName,
			"ProfileImage":  userData.ProfileImage,
			"WishlistCount": wishlistCount,
			"CartCount":     cartCount,
		})
	})
}
