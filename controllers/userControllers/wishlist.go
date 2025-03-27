package controllers

// import (
// 	"net/http"
// 	"strconv"

// 	"github.com/Sojil8/eCommerce-silver/database"
// 	"github.com/Sojil8/eCommerce-silver/models/adminModels"
// 	"github.com/Sojil8/eCommerce-silver/models/userModels"
// 	"github.com/gin-gonic/gin"
// )

// func ShowWishlist(c *gin.Context) {
// 	userID, exist := c.Get("id")
// 	if !exist {
// 		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
// 		return
// 	}

// 	var wishlistItems userModels.Wishlist
// 	if err := database.DB.Preload("Products").Where("user_id = ?", userID).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load wishlist"})
// 		return
// 	}


// 	c.HTML(http.StatusOK, "wishlist.html", gin.H{
// 		"Wishlist": wishlistItems,
// 		"Title":    "My Wishlist",
// 	})

// }

// func AddToWishlist(c *gin.Context) {
// 	userID, exits := c.Get("id")
// 	if !exits {
// 		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
// 		return
// 	}

// 	productIDStr := c.Param("id")
// 	productID, err := strconv.Atoi(productIDStr)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
// 		return
// 	}

// 	var product adminModels.Product
// 	if err := database.DB.First(&product, productID).Error; err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
// 		return
// 	}

// 	var existingWishlist userModels.Wishlist
// 	if err := database.DB.Where("user_id = ? AND product_id = ?", userID, productID).First(&existingWishlist).Error; err == nil {
// 		if err := database.DB.Delete(&existingWishlist).Error; err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"status": "ERROR", "error": "Failed to remove from wishlist"})
// 			return
// 		}
// 		c.JSON(http.StatusOK, gin.H{"status": "REMOVED", "message": "Product removed from wishlist"})
// 		return
// 	}

// 	wishlistItem := userModels.Wishlist{
// 		UserID:    userID.(uint),
// 		ProductID: uint(productID),
// 	}

// 	if err := database.DB.Create(&wishlistItem).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add to wishlist"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"message": "Product added to wishlist"})
// }

// func RemoveWishList(c *gin.Context) {
// 	userID, exist := c.Get("id")
// 	if !exist {
// 		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
// 		return
// 	}

// 	productID := c.Param("id")
// 	productStr, err := strconv.Atoi(productID)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
// 		return
// 	}

// 	if err := database.DB.Where("user_id = ? AND product_id = ?", userID, productStr).Delete(&userModels.Wishlist{}).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove from wishlist"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"message": "Product removed from wishlist"})

// }
