package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

	for i := range wishlistItems {
		product := &wishlistItems[i].Product

		// Get the first variant to calculate base price (you might want to handle this differently)
		variantPrice := 0.0
		if len(product.Variants) > 0 {
			variantPrice = product.Variants[0].ExtraPrice
		}

		offer := helper.GetBestOfferForProduct(product, variantPrice)
		product.Price = offer.DiscountedPrice // Update the product price with offer
		product.OriginalPrice = offer.OriginalPrice
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

	if c.Request.Header.Get("Accept") == "application/json" {
		// Return JSON data
		c.JSON(http.StatusOK, gin.H{
			"status":   "success",
			"message":  "Wishlist loaded successfully",
			"wishlist": wishlistItems,
		})
		return
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

	productIDStr := c.Param("id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	c.Writer.Header().Set("Content-Type", "application/json")

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

type AddProductsToCart struct {
	productID adminModels.Product
}

func AddAllToCartFromWishlist(c *gin.Context) {
	userID, exist := c.Get("id")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user's wishlist items
	var wishlistItems []userModels.Wishlist
	if err := database.DB.Preload("Product.Variants").Where("user_id = ?", userID).Find(&wishlistItems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load wishlist"})
		return
	}

	if len(wishlistItems) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message":      "No items in wishlist",
			"added_count":  0,
			"failed_count": 0,
		})
		return
	}

	// Get user's cart
	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).First(&cart).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create a new cart if it doesn't exist
			cart = userModels.Cart{UserID: userID.(uint)}
			if err := database.DB.Create(&cart).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cart"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cart"})
			return
		}
	}

	// Start a transaction to ensure data consistency
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Track added and failed items
	addedItems := make([]uint, 0)
	failedItems := make([]uint, 0)
	wishlistIDsToRemove := make([]uint, 0)

	// Add each wishlist item to cart
	for _, item := range wishlistItems {
		// Check if product is in stock
		if !item.Product.InStock {
			failedItems = append(failedItems, item.ProductID)
			continue
		}

		// Get the first available variant
		var variantID uint
		for _, variant := range item.Product.Variants {
			if variant.Stock > 0 {
				variantID = variant.ID
				break
			}
		}

		if variantID == 0 {
			failedItems = append(failedItems, item.ProductID)
			continue
		}

		// Check if item already exists in cart
		var existingCartItem userModels.CartItem
		err := tx.Where("cart_id = ? AND product_id = ? AND variants_id = ?", cart.ID, item.ProductID, variantID).First(&existingCartItem).Error

		if err == nil {
			// Item exists, update quantity
			if err := tx.Model(&existingCartItem).Update("quantity", gorm.Expr("quantity + ?", 1)).Error; err != nil {
				failedItems = append(failedItems, item.ProductID)
				continue
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new cart item
			cartItem := userModels.CartItem{
				CartID:     cart.ID,
				ProductID:  item.ProductID,
				VariantsID: variantID,
				Quantity:   1,
			}

			if err := tx.Create(&cartItem).Error; err != nil {
				failedItems = append(failedItems, item.ProductID)
				continue
			}
		} else {
			// Other database error
			failedItems = append(failedItems, item.ProductID)
			continue
		}

		// Mark this wishlist item for removal
		addedItems = append(addedItems, item.ProductID)
		wishlistIDsToRemove = append(wishlistIDsToRemove, item.ID)
	}

	// Remove successfully added items from wishlist
	if len(wishlistIDsToRemove) > 0 {
		if err := tx.Where("id IN ?", wishlistIDsToRemove).Delete(&userModels.Wishlist{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove items from wishlist"})
			return
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete the operation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Items added to cart",
		"added_count":  len(addedItems),
		"failed_count": len(failedItems),
		"added_items":  addedItems,
		"failed_items": failedItems,
	})
}

// Add this to your wishlist controller
func GetVariantPrice(c *gin.Context) {
	var request struct {
		ProductID uint `json:"product_id"`
		VariantID uint `json:"variant_id"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get product with offers
	var product adminModels.Product
	if err := database.DB.
		Preload("Offers", "is_active = ? AND start_date <= ? AND end_date >= ?", true, time.Now(), time.Now()).
		First(&product, request.ProductID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Get variant
	var variant adminModels.Variants
	if err := database.DB.First(&variant, request.VariantID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Variant not found"})
		return
	}

	// Calculate price with variant extra price
	// basePrice := product.Price + variant.ExtraPrice
	originalPrice := product.OriginalPrice + variant.ExtraPrice

	// Apply offers if any
	offer := helper.GetBestOfferForProduct(&product, variant.ExtraPrice)

	response := gin.H{
		"original_price":   originalPrice,
		"discounted_price": offer.DiscountedPrice,
		"has_offer":        offer.IsOfferApplied,
		"offer_name":       offer.OfferName,
	}

	c.JSON(http.StatusOK, response)
}
