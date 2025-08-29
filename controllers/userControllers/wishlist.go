package controllers

import (
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
)

func ShowWishlist(c *gin.Context) {
	userID, exist := c.Get("id")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var wishlistItems []userModels.Wishlist
	if err := database.DB.Preload("Product.Variants").Where("user_id = ?", userID).Find(&wishlistItems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load wishlist"})
		return
	}
	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "wishlist.html", gin.H{
			"status":        "success",
			"Wishlist":      wishlistItems,
			"Title":         "My Wishlist",
			"UserName":      "Guest",
			"WishlistCount": 0,
			"CartCount":     0,
			"ProfileImage":  "",
		})
		return
	}

	userData := user.(userModels.Users)
	userNameStr := userName.(string)

	var wishlistCount, cartCount int64
	if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		cartCount = 0
	}

	c.HTML(http.StatusOK, "wishlist.html", gin.H{
		"Wishlist":      wishlistItems,
		"Title":         "My Wishlist",
		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
	})

}

func AddToWishlist(c *gin.Context) {
	userID, exits := c.Get("id")
	if !exits {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	productIDStr := c.Param("id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var product adminModels.Product
	if err := database.DB.First(&product, uint(productID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var existingWishlist userModels.Wishlist
	if err := database.DB.Where("user_id = ? AND product_id = ?", userID, productID).First(&existingWishlist).Error; err == nil {
		if err := database.DB.Delete(&existingWishlist).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "ERROR", "error": "Failed to remove from wishlist"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "REMOVED", "message": "Product removed from wishlist"})
		return
	}

	wishlistItem := userModels.Wishlist{
		UserID:    userID.(uint),
		ProductID: uint(productID),
	}

	if err := database.DB.Create(&wishlistItem).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add to wishlist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product added to wishlist"})
}
func RemoveWishList(c *gin.Context) {
	userID, exist := c.Get("id")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Change this to expect product ID instead of wishlist ID
	productIDStr := c.Param("id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	c.Writer.Header().Set("Content-Type", "application/json")

	// Change the query to use product_id instead of wishlist id
	result := database.DB.Where("product_id = ? AND user_id = ?", productID, userID).Delete(&userModels.Wishlist{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove from wishlist", "details": result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Wishlist item not found or not authorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product removed from wishlist"})
}
