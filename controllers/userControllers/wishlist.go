package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func ShowWishlist(c *gin.Context) {
	pkg.Log.Info("Starting wishlist page retrieval")

	userID, exist := c.Get("id")
	if !exist {
		pkg.Log.Warn("User not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	var wishlistItems []userModels.Wishlist
	if err := database.DB.Preload("Product.Variants").Where("user_id = ?", userID).Find(&wishlistItems).Error; err != nil {
		pkg.Log.Error("Failed to load wishlist",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load wishlist"})
		return
	}

	pkg.Log.Debug("Fetched wishlist items",
		zap.Uint("user_id", userIDUint),
		zap.Int("wishlist_count", len(wishlistItems)))

	for i := range wishlistItems {
		product := &wishlistItems[i].Product

		variantPrice := 0.0
		if len(product.Variants) > 0 {
			variantPrice = product.Variants[0].ExtraPrice
		}

		offer := helper.GetBestOfferForProduct(product, variantPrice)
		product.Price = offer.DiscountedPrice
		product.OriginalPrice = offer.OriginalPrice

		pkg.Log.Debug("Applied offer to wishlist product",
			zap.Uint("user_id", userIDUint),
			zap.Uint("product_id", product.ID),
			zap.Float64("discounted_price", offer.DiscountedPrice),
			zap.Float64("original_price", offer.OriginalPrice),
			zap.Bool("has_offer", offer.IsOfferApplied))
	}

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		pkg.Log.Info("Rendering wishlist page for guest user",
			zap.Int("wishlist_count", len(wishlistItems)))

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
		pkg.Log.Warn("Failed to fetch wishlist count",
			zap.Uint("user_id", userData.ID),
			zap.Error(err))
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		pkg.Log.Warn("Failed to fetch cart count",
			zap.Uint("user_id", userData.ID),
			zap.Error(err))
		cartCount = 0
	}

	pkg.Log.Debug("Fetched user counts",
		zap.Uint("user_id", userData.ID),
		zap.String("user_name", userNameStr),
		zap.Int64("wishlist_count", wishlistCount),
		zap.Int64("cart_count", cartCount))

	if c.Request.Header.Get("Accept") == "application/json" {
		pkg.Log.Info("Returning wishlist as JSON",
			zap.Uint("user_id", userData.ID),
			zap.Int("wishlist_count", len(wishlistItems)))
		c.JSON(http.StatusOK, gin.H{
			"status":   "success",
			"message":  "Wishlist loaded successfully",
			"wishlist": wishlistItems,
		})
		return
	}

	pkg.Log.Info("Rendering wishlist page for authenticated user",
		zap.Uint("user_id", userData.ID),
		zap.String("user_name", userNameStr),
		zap.Int("wishlist_count", len(wishlistItems)),
		zap.Int64("wishlist_count_total", wishlistCount),
		zap.Int64("cart_count", cartCount))

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
	pkg.Log.Info("Starting add to wishlist process")

	userID, exist := c.Get("id")
	if !exist {
		pkg.Log.Warn("User not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	productIDStr := c.Param("id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		pkg.Log.Warn("Invalid product ID",
			zap.String("product_id_str", productIDStr),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var product adminModels.Product
	if err := database.DB.First(&product, uint(productID)).Error; err != nil {
		pkg.Log.Warn("Product not found",
			zap.Uint("user_id", userIDUint),
			zap.Uint("product_id", uint(productID)),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var existingWishlist userModels.Wishlist
	if err := database.DB.Where("user_id = ? AND product_id = ?", userID, productID).First(&existingWishlist).Error; err == nil {
		if err := database.DB.Delete(&existingWishlist).Error; err != nil {
			pkg.Log.Error("Failed to remove existing wishlist item",
				zap.Uint("user_id", userIDUint),
				zap.Uint("product_id", uint(productID)),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"status": "ERROR", "error": "Failed to remove from wishlist"})
			return
		}
		pkg.Log.Info("Removed existing product from wishlist",
			zap.Uint("user_id", userIDUint),
			zap.Uint("product_id", uint(productID)))
		c.JSON(http.StatusOK, gin.H{"status": "REMOVED", "message": "Product removed from wishlist"})
		return
	}

	wishlistItem := userModels.Wishlist{
		UserID:    userIDUint,
		ProductID: uint(productID),
	}

	if err := database.DB.Create(&wishlistItem).Error; err != nil {
		pkg.Log.Error("Failed to add to wishlist",
			zap.Uint("user_id", userIDUint),
			zap.Uint("product_id", uint(productID)),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add to wishlist"})
		return
	}

	pkg.Log.Info("Product added to wishlist",
		zap.Uint("user_id", userIDUint),
		zap.Uint("product_id", uint(productID)))

	c.JSON(http.StatusOK, gin.H{"message": "Product added to wishlist"})
}

func RemoveWishList(c *gin.Context) {
	pkg.Log.Info("Starting remove from wishlist process")

	userID, exist := c.Get("id")
	if !exist {
		pkg.Log.Warn("User not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	productIDStr := c.Param("id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		pkg.Log.Warn("Invalid product ID",
			zap.String("product_id_str", productIDStr),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	c.Writer.Header().Set("Content-Type", "application/json")

	result := database.DB.Where("product_id = ? AND user_id = ?", productID, userID).Delete(&userModels.Wishlist{})
	if result.Error != nil {
		pkg.Log.Error("Failed to remove from wishlist",
			zap.Uint("user_id", userIDUint),
			zap.Uint("product_id", uint(productID)),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove from wishlist", "details": result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		pkg.Log.Warn("Wishlist item not found or not authorized",
			zap.Uint("user_id", userIDUint),
			zap.Uint("product_id", uint(productID)))
		c.JSON(http.StatusNotFound, gin.H{"error": "Wishlist item not found or not authorized"})
		return
	}

	pkg.Log.Info("Product removed from wishlist",
		zap.Uint("user_id", userIDUint),
		zap.Uint("product_id", uint(productID)))

	c.JSON(http.StatusOK, gin.H{"message": "Product removed from wishlist"})
}

func AddAllToCartFromWishlist(c *gin.Context) {
	pkg.Log.Info("Starting add all to cart from wishlist process")

	userID, exist := c.Get("id")
	if !exist {
		pkg.Log.Warn("User not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	var wishlistItems []userModels.Wishlist
	if err := database.DB.Preload("Product.Variants").Where("user_id = ?", userID).Find(&wishlistItems).Error; err != nil {
		pkg.Log.Error("Failed to load wishlist",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load wishlist"})
		return
	}

	if len(wishlistItems) == 0 {
		pkg.Log.Info("No items in wishlist",
			zap.Uint("user_id", userIDUint))
		c.JSON(http.StatusOK, gin.H{
			"message":      "No items in wishlist",
			"added_count":  0,
			"failed_count": 0,
		})
		return
	}

	pkg.Log.Debug("Fetched wishlist items for cart addition",
		zap.Uint("user_id", userIDUint),
		zap.Int("wishlist_count", len(wishlistItems)))

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).First(&cart).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cart = userModels.Cart{UserID: userIDUint}
			if err := database.DB.Create(&cart).Error; err != nil {
				pkg.Log.Error("Failed to create cart",
					zap.Uint("user_id", userIDUint),
					zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cart"})
				return
			}
			pkg.Log.Info("Created new cart",
				zap.Uint("user_id", userIDUint),
				zap.Uint("cart_id", cart.ID))
		} else {
			pkg.Log.Error("Failed to get cart",
				zap.Uint("user_id", userIDUint),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cart"})
			return
		}
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			pkg.Log.Error("Panic during cart addition, rolling back transaction",
				zap.Uint("user_id", userIDUint),
				zap.Any("panic", r))
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unexpected error during cart addition"})
			return
		}
	}()

	addedItems := make([]uint, 0)
	failedItems := make([]uint, 0)
	wishlistIDsToRemove := make([]uint, 0)

	for _, item := range wishlistItems {
		if !item.Product.InStock {
			pkg.Log.Warn("Product not in stock",
				zap.Uint("user_id", userIDUint),
				zap.Uint("product_id", item.ProductID))
			failedItems = append(failedItems, item.ProductID)
			continue
		}

		var variantID uint
		for _, variant := range item.Product.Variants {
			if variant.Stock > 0 {
				variantID = variant.ID
				break
			}
		}

		if variantID == 0 {
			pkg.Log.Warn("No valid variant with stock",
				zap.Uint("user_id", userIDUint),
				zap.Uint("product_id", item.ProductID))
			failedItems = append(failedItems, item.ProductID)
			continue
		}

		var existingCartItem userModels.CartItem
		err := tx.Where("cart_id = ? AND product_id = ? AND variants_id = ?", cart.ID, item.ProductID, variantID).First(&existingCartItem).Error

		if err == nil {
			if err := tx.Model(&existingCartItem).Update("quantity", gorm.Expr("quantity + ?", 1)).Error; err != nil {
				pkg.Log.Error("Failed to update cart item quantity",
					zap.Uint("user_id", userIDUint),
					zap.Uint("product_id", item.ProductID),
					zap.Uint("variant_id", variantID),
					zap.Error(err))
				failedItems = append(failedItems, item.ProductID)
				continue
			}
			pkg.Log.Debug("Updated existing cart item quantity",
				zap.Uint("user_id", userIDUint),
				zap.Uint("product_id", item.ProductID),
				zap.Uint("variant_id", variantID))
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			cartItem := userModels.CartItem{
				CartID:     cart.ID,
				ProductID:  item.ProductID,
				VariantsID: variantID,
				Quantity:   1,
			}

			if err := tx.Create(&cartItem).Error; err != nil {
				pkg.Log.Error("Failed to create cart item",
					zap.Uint("user_id", userIDUint),
					zap.Uint("product_id", item.ProductID),
					zap.Uint("variant_id", variantID),
					zap.Error(err))
				failedItems = append(failedItems, item.ProductID)
				continue
			}
			pkg.Log.Debug("Created new cart item",
				zap.Uint("user_id", userIDUint),
				zap.Uint("product_id", item.ProductID),
				zap.Uint("variant_id", variantID))
		} else {
			pkg.Log.Error("Failed to check existing cart item",
				zap.Uint("user_id", userIDUint),
				zap.Uint("product_id", item.ProductID),
				zap.Uint("variant_id", variantID),
				zap.Error(err))
			failedItems = append(failedItems, item.ProductID)
			continue
		}

		addedItems = append(addedItems, item.ProductID)
		wishlistIDsToRemove = append(wishlistIDsToRemove, item.ID)
	}

	if len(wishlistIDsToRemove) > 0 {
		if err := tx.Where("id IN ?", wishlistIDsToRemove).Delete(&userModels.Wishlist{}).Error; err != nil {
			pkg.Log.Error("Failed to remove items from wishlist",
				zap.Uint("user_id", userIDUint),
				zap.Uints("wishlist_ids", wishlistIDsToRemove),
				zap.Error(err))
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove items from wishlist"})
			return
		}
		pkg.Log.Debug("Removed wishlist items",
			zap.Uint("user_id", userIDUint),
			zap.Uints("wishlist_ids", wishlistIDsToRemove))
	}

	if err := tx.Commit().Error; err != nil {
		pkg.Log.Error("Failed to commit transaction",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete the operation"})
		return
	}

	pkg.Log.Info("Items added to cart from wishlist",
		zap.Uint("user_id", userIDUint),
		zap.Int("added_count", len(addedItems)),
		zap.Int("failed_count", len(failedItems)),
		zap.Uints("added_items", addedItems),
		zap.Uints("failed_items", failedItems))

	c.JSON(http.StatusOK, gin.H{
		"message":      "Items added to cart",
		"added_count":  len(addedItems),
		"failed_count": len(failedItems),
		"added_items":  addedItems,
		"failed_items": failedItems,
	})
}

func GetVariantPrice(c *gin.Context) {
	pkg.Log.Info("Starting get variant price process")

	var request struct {
		ProductID uint `json:"product_id"`
		VariantID uint `json:"variant_id"`
	}

	if err := c.BindJSON(&request); err != nil {
		pkg.Log.Error("Failed to bind request",
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	pkg.Log.Debug("Processing variant price request",
		zap.Uint("product_id", request.ProductID),
		zap.Uint("variant_id", request.VariantID))

	var product adminModels.Product
	if err := database.DB.
		Preload("Offers", "is_active = ? AND start_date <= ? AND end_date >= ?", true, time.Now(), time.Now()).
		First(&product, request.ProductID).Error; err != nil {
		pkg.Log.Warn("Product not found",
			zap.Uint("product_id", request.ProductID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var variant adminModels.Variants
	if err := database.DB.First(&variant, request.VariantID).Error; err != nil {
		pkg.Log.Warn("Variant not found",
			zap.Uint("variant_id", request.VariantID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Variant not found"})
		return
	}

	originalPrice := product.OriginalPrice + variant.ExtraPrice
	offer := helper.GetBestOfferForProduct(&product, variant.ExtraPrice)

	pkg.Log.Info("Retrieved variant price",
		zap.Uint("product_id", request.ProductID),
		zap.Uint("variant_id", request.VariantID),
		zap.Float64("original_price", originalPrice),
		zap.Float64("discounted_price", offer.DiscountedPrice),
		zap.Bool("has_offer", offer.IsOfferApplied),
		zap.String("offer_name", offer.OfferName))

	response := gin.H{
		"original_price":   originalPrice,
		"discounted_price": offer.DiscountedPrice,
		"has_offer":        offer.IsOfferApplied,
		"offer_name":       offer.OfferName,
	}

	c.JSON(http.StatusOK, response)
}