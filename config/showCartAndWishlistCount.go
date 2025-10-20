package config

import (
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func SetupRoutes(r *gin.Engine, db *gorm.DB) {
	pkg.Log.Info("Setting up routes for Gin engine")

	authenticated := r.Group("/", middleware.Authenticate("jwt_token", "User", "/login"))
	authenticated.GET("/home", func(c *gin.Context) {
		pkg.Log.Info("Handling GET /home request")

		user, exists := c.Get("user")
		if !exists {
			pkg.Log.Error("User not found in context")
			c.JSON(500, gin.H{"error": "User not found in context"})
			return
		}

		userData, ok := user.(userModels.Users)
		if !ok {
			pkg.Log.Error("Failed to cast user to userModels.Users")
			c.JSON(500, gin.H{"error": "Invalid user data"})
			return
		}
		pkg.Log.Debug("User retrieved from context", zap.Uint("user_id", userData.ID), zap.String("username", userData.UserName))

		var wishlistCount, cartCount int64
		if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
			pkg.Log.Error("Failed to query wishlist count", zap.Uint("user_id", userData.ID), zap.Error(err))
			wishlistCount = 0
		} else {
			pkg.Log.Debug("Wishlist count retrieved", zap.Uint("user_id", userData.ID), zap.Int64("count", wishlistCount))
		}

		if err := database.DB.Model(&userModels.Cart{}).Where("user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
			pkg.Log.Error("Failed to query cart count", zap.Uint("user_id", userData.ID), zap.Error(err))
			cartCount = 0
		} else {
			pkg.Log.Debug("Cart count retrieved", zap.Uint("user_id", userData.ID), zap.Int64("count", cartCount))
		}

		pkg.Log.Info("Rendering home.html", zap.String("username", userData.UserName), zap.Int64("wishlist_count", wishlistCount), zap.Int64("cart_count", cartCount))
		c.HTML(200, "home.html", gin.H{
			"UserName":      userData.UserName,
			"ProfileImage":  userData.ProfileImage,
			"WishlistCount": wishlistCount,
			"CartCount":     cartCount,
		})
	})
}