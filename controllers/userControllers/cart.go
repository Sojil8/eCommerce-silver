package controllers

import (
	"fmt"
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg" // Import your logger package
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap" // For zap fields
	"gorm.io/gorm"
)

const MAX_QUANTITY_PER_PRODUCT = 5

func GetCart(c *gin.Context) {
	userID := helper.FetchUserID(c)
	var cart userModels.Cart

	// Log the start of the GetCart operation
	pkg.Log.Debug("Fetching cart for user", zap.Uint("user_id", userID))

	err := database.DB.Where("user_id = ?", userID).Preload("CartItems.Product").Preload("CartItems.Variants").First(&cart).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			pkg.Log.Info("Cart not found, creating new cart", zap.Uint("user_id", userID))
			cart = userModels.Cart{UserID: userID}
			if err := database.DB.Create(&cart).Error; err != nil {
				pkg.Log.Error("Failed to create cart", zap.Uint("user_id", userID), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cart"})
				return
			}
		} else {
			pkg.Log.Error("Failed to fetch cart", zap.Uint("user_id", userID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cart"})
			return
		}
	}

	var allCartItems []userModels.CartItem
	var inStockCartItems []userModels.CartItem
	var outOfStockCartItems []userModels.CartItem
	var invalidItems []string

	totalPrice := 0.0
	totalDiscount := 0.0
	outOfStockCount := 0
	isInStock := false

	for _, item := range cart.CartItems {
		var product adminModels.Product
		if err := database.DB.Preload("Variants").First(&product, item.ProductID).Error; err != nil {
			pkg.Log.Warn("Product not found, removing cart item",
				zap.Uint("product_id", item.ProductID),
				zap.Uint("cart_id", cart.ID),
				zap.Error(err))
			database.DB.Delete(&item)
			invalidItems = append(invalidItems, fmt.Sprintf("Product %d", item.ProductID))
			continue
		}

		var category adminModels.Category
		if !product.IsListed ||
			database.DB.Where("category_name = ? AND status = ?", product.CategoryName, true).First(&category).Error != nil {
			pkg.Log.Warn("Product unlisted or invalid category",
				zap.Uint("product_id", item.ProductID),
				zap.String("category_name", product.CategoryName))
			invalidItems = append(invalidItems, fmt.Sprintf("Product %d (unlisted or invalid category)", item.ProductID))
			continue
		}

		var variant adminModels.Variants
		if err := database.DB.Where("deleted_at IS NULL").First(&variant, item.VariantsID).Error; err != nil || variant.ProductID != item.ProductID {
			pkg.Log.Warn("Invalid or missing variant, skipping cart item",
				zap.Uint("variant_id", item.VariantsID),
				zap.Uint("product_id", item.ProductID),
				zap.Error(err))
			invalidItems = append(invalidItems, fmt.Sprintf("Product %d (Variant %d)", item.ProductID, item.VariantsID))
			continue
		}

		item.Product = product
		item.Variants = variant

		isInStock = variant.Stock > 0 && variant.Stock >= item.Quantity
		item.Product.InStock = isInStock

		// Replace log.Printf with zap
		pkg.Log.Debug("Processing cart item",
			zap.Uint("product_id", item.ProductID),
			zap.Uint("variant_id", item.VariantsID),
			zap.String("product_name", product.ProductName),
			zap.Int("stock", int(variant.Stock)),
			zap.Uint("quantity", item.Quantity),
			zap.Bool("in_stock", isInStock))

		variantExtraPrice := variant.ExtraPrice
		offer := helper.GetBestOfferForProduct(&product, variantExtraPrice)

		item.Price = product.Price + variantExtraPrice
		item.DiscountedPrice = offer.DiscountedPrice
		item.RegularPrice = product.Price + variantExtraPrice
		item.OfferDiscountPercentage = offer.DiscountPercentage
		item.OfferName = offer.OfferName
		item.IsOfferApplied = offer.IsOfferApplied
		item.SalePrice = item.DiscountedPrice * float64(item.Quantity)

		if err := database.DB.Save(&item).Error; err != nil {
			pkg.Log.Error("Failed to update cart item",
				zap.Uint("cart_id", cart.ID),
				zap.Uint("product_id", item.ProductID),
				zap.Uint("variant_id", item.VariantsID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart item"})
			return
		}

		allCartItems = append(allCartItems, item)

		if isInStock {
			inStockCartItems = append(inStockCartItems, item)
			totalPrice += item.SalePrice

			if item.IsOfferApplied {
				discountPerItem := (item.RegularPrice - item.DiscountedPrice) * float64(item.Quantity)
				totalDiscount += discountPerItem
			}
		} else {
			outOfStockCartItems = append(outOfStockCartItems, item)
			outOfStockCount++
		}
	}

	cart.TotalPrice = totalPrice
	if err := database.DB.Save(&cart).Error; err != nil {
		pkg.Log.Error("Failed to update cart total",
			zap.Uint("cart_id", cart.ID),
			zap.Float64("total_price", totalPrice),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart total"})
		return
	}

	// Log cart summary
	pkg.Log.Info("Cart summary",
		zap.Uint("user_id", userID),
		zap.Uint("cart_id", cart.ID),
		zap.Int("total_items", len(allCartItems)),
		zap.Int("in_stock_items", len(inStockCartItems)),
		zap.Int("out_of_stock_items", len(outOfStockCartItems)),
		zap.Float64("total_price", totalPrice),
		zap.Float64("total_discount", totalDiscount))

	userr, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		pkg.Log.Debug("Rendering cart for guest user", zap.Uint("user_id", userID))
		c.HTML(http.StatusOK, "cart.html", gin.H{
			"CartItems":           allCartItems,
			"InStockCartItems":    inStockCartItems,
			"OutOfStockCartItems": outOfStockCartItems,
			"TotalPrice":          totalPrice,
			"TotalDiscount":       totalDiscount,
			"CartItemCount":       len(inStockCartItems),
			"OutOfStockCount":     outOfStockCount,
			"TotalItemCount":      len(allCartItems),
			"CanCheckout":         len(inStockCartItems) > 0 && outOfStockCount == 0,
			"status":              "success",
			"UserName":            "Guest",
			"WishlistCount":       0,
			"CartCount":           0,
			"ProfileImage":        "",
			"InvalidItems":        invalidItems,
		})
		return
	}

	userData := userr.(userModels.Users)
	userNameStr := userName.(string)

	var wishlistCount, cartCount int64
	if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
		pkg.Log.Warn("Failed to fetch wishlist count", zap.Uint("user_id", userData.ID), zap.Error(err))
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		pkg.Log.Warn("Failed to fetch cart count", zap.Uint("user_id", userData.ID), zap.Error(err))
		cartCount = 0
	}

	pkg.Log.Debug("Rendering cart for authenticated user",
		zap.Uint("user_id", userData.ID),
		zap.String("user_name", userNameStr),
		zap.Int64("wishlist_count", wishlistCount),
		zap.Int64("cart_count", cartCount))

	c.HTML(http.StatusOK, "cart.html", gin.H{
		"CartItems":           allCartItems,
		"InStockCartItems":    inStockCartItems,
		"OutOfStockCartItems": outOfStockCartItems,
		"TotalPrice":          totalPrice,
		"TotalDiscount":       totalDiscount,
		"CartItemCount":       len(inStockCartItems),
		"OutOfStockCount":     outOfStockCount,
		"TotalItemCount":      len(allCartItems),
		"CanCheckout":         len(inStockCartItems) > 0 && outOfStockCount == 0,
		"HasStock":            isInStock,
		"UserName":            userNameStr,
		"ProfileImage":        userData.ProfileImage,
		"WishlistCount":       wishlistCount,
		"CartCount":           cartCount,
		"InvalidItems":        invalidItems,
	})
}

type RequestCartItem struct {
	ProductID uint `json:"product_id"`
	VariantID uint `json:"variant_id"`
	Quantity  uint `json:"quantity"`
}

type CartInput struct {
	WishlistID *uint `json:"wishlist_id"`
	ProductID  *uint `json:"product_id"`
	VariantID  *uint `json:"variant_id"`
	Quantity   uint  `json:"quantity"`
}

func AddToCart(c *gin.Context) {
	userID, _ := c.Get("id")

	var req CartInput
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Log.Error("Invalid request payload", zap.Any("user_id", userID), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	// Log request details
	pkg.Log.Debug("AddToCart request",
		zap.Any("user_id", userID),
		zap.Any("request", req))

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var productID uint

		if req.WishlistID != nil {
			var wishlist userModels.Wishlist
			if err := tx.First(&wishlist, *req.WishlistID).Error; err != nil {
				pkg.Log.Error("Wishlist item not found",
					zap.Any("user_id", userID),
					zap.Uint("wishlist_id", *req.WishlistID),
					zap.Error(err))
				return gin.Error{Err: err, Meta: gin.H{"error": "Wishlist item not found"}}
			}
			if wishlist.UserID != userID.(uint) {
				pkg.Log.Warn("Unauthorized access to wishlist item",
					zap.Any("user_id", userID),
					zap.Uint("wishlist_id", *req.WishlistID))
				return gin.Error{Meta: gin.H{"error": "Unauthorized access to wishlist item"}}
			}
			productID = wishlist.ProductID
			if err := tx.Where("user_id = ? AND id = ?", userID, *req.WishlistID).
				Delete(&userModels.Wishlist{}).Error; err != nil {
				pkg.Log.Error("Failed to delete wishlist item",
					zap.Any("user_id", userID),
					zap.Uint("wishlist_id", *req.WishlistID),
					zap.Error(err))
				return err
			}
			pkg.Log.Info("Removed wishlist item for cart addition",
				zap.Any("user_id", userID),
				zap.Uint("wishlist_id", *req.WishlistID))
		} else if req.ProductID != nil {
			productID = *req.ProductID
		} else {
			pkg.Log.Error("Missing product_id or wishlist_id",
				zap.Any("user_id", userID))
			return gin.Error{Meta: gin.H{"error": "Either product_id or wishlist_id is required"}}
		}

		// Load product with variants
		var product adminModels.Product
		if err := tx.Preload("Variants", "deleted_at IS NULL").Preload("Offers").First(&product, productID).Error; err != nil {
			pkg.Log.Error("Product not found",
				zap.Any("user_id", userID),
				zap.Uint("product_id", productID),
				zap.Error(err))
			return gin.Error{Err: err, Meta: gin.H{"error": "Product not found"}}
		}

		pkg.Log.Debug("Product loaded",
			zap.Uint("product_id", product.ID),
			zap.String("product_name", product.ProductName),
			zap.Int("variants_count", len(product.Variants)))

		// Check category
		var category adminModels.Category
		if err := tx.Where("category_name = ?", product.CategoryName).First(&category).Error; err != nil || !category.Status {
			pkg.Log.Warn("Product category is not available",
				zap.Uint("product_id", product.ID),
				zap.String("category_name", product.CategoryName),
				zap.Error(err))
			return gin.Error{Err: err, Meta: gin.H{"error": "Product category is not available"}}
		}

		// Check if product is listed
		if !product.IsListed {
			pkg.Log.Warn("Product is not available",
				zap.Uint("product_id", product.ID))
			return gin.Error{Meta: gin.H{"error": "Product is not available"}}
		}

		// Handle variant selection
		var variant adminModels.Variants

		if req.VariantID == nil || *req.VariantID == 0 {
			// No variant specified, use first available variant
			if len(product.Variants) == 0 {
				// If no variants loaded via Preload, try to find any variants
				var variants []adminModels.Variants
				if err := tx.Where("product_id = ? AND deleted_at IS NULL", productID).Find(&variants).Error; err != nil {
					pkg.Log.Error("Error finding variants",
						zap.Uint("product_id", productID),
						zap.Error(err))
					return gin.Error{Meta: gin.H{"error": "No variants available for this product"}}
				}
				if len(variants) == 0 {
					pkg.Log.Warn("No variants available for product",
						zap.Uint("product_id", productID))
					return gin.Error{Meta: gin.H{"error": "No variants available for this product"}}
				}
				variant = variants[0]
				pkg.Log.Debug("Selected first variant from direct query",
					zap.Uint("variant_id", variant.ID))
			} else {
				variant = product.Variants[0]
				pkg.Log.Debug("Selected first variant from preload",
					zap.Uint("variant_id", variant.ID))
			}
		} else {
			// Specific variant requested
			if err := tx.Where("id = ? AND product_id = ? AND deleted_at IS NULL", *req.VariantID, productID).First(&variant).Error; err != nil {
				pkg.Log.Warn("Invalid variant",
					zap.Any("user_id", userID),
					zap.Uint("variant_id", *req.VariantID),
					zap.Uint("product_id", productID),
					zap.Error(err))
				return gin.Error{Err: err, Meta: gin.H{"error": "Invalid or missing variant"}}
			}
			pkg.Log.Debug("Selected specified variant",
				zap.Uint("variant_id", variant.ID))
		}

		// Check stock
		if variant.Stock < req.Quantity {
			pkg.Log.Warn("Not enough stock available",
				zap.Uint("variant_id", variant.ID),
				zap.Uint("requested_quantity", req.Quantity),
				zap.Int("available_stock", int(variant.Stock)))
			return gin.Error{Meta: gin.H{"error": "Not enough stock available"}}
		}

		if req.Quantity > MAX_QUANTITY_PER_PRODUCT {
			pkg.Log.Warn("Quantity exceeds maximum limit",
				zap.Uint("variant_id", variant.ID),
				zap.Uint("requested_quantity", req.Quantity),
				zap.Int("max_quantity", MAX_QUANTITY_PER_PRODUCT))
			return gin.Error{Meta: gin.H{"error": "Quantity exceeds maximum limit"}}
		}

		// Get or create cart
		var cart userModels.Cart
		if err := tx.Where("user_id = ?", userID).Preload("CartItems").
			FirstOrCreate(&cart, userModels.Cart{UserID: userID.(uint)}).Error; err != nil {
			pkg.Log.Error("Failed to get or create cart",
				zap.Any("user_id", userID),
				zap.Error(err))
			return err
		}

		// Calculate pricing
		variantExtraPrice := variant.ExtraPrice
		offer := helper.GetBestOfferForProduct(&product, variantExtraPrice)

		// Check if item already exists in cart
		for i, item := range cart.CartItems {
			if item.ProductID == productID && item.VariantsID == variant.ID {
				newQnty := item.Quantity + req.Quantity
				if newQnty > MAX_QUANTITY_PER_PRODUCT || newQnty > variant.Stock {
					pkg.Log.Warn("Quantity limit exceeded for existing cart item",
						zap.Uint("product_id", productID),
						zap.Uint("variant_id", variant.ID),
						zap.Uint("new_quantity", newQnty),
						zap.Int("max_quantity", MAX_QUANTITY_PER_PRODUCT),
						zap.Int("available_stock", int(variant.Stock)))
					return gin.Error{Meta: gin.H{"error": "Quantity limit exceeded"}}
				}

				cart.CartItems[i].Quantity = newQnty
				cart.CartItems[i].Price = product.Price + variantExtraPrice
				cart.CartItems[i].DiscountedPrice = offer.DiscountedPrice
				cart.CartItems[i].RegularPrice = product.Price + variantExtraPrice
				cart.CartItems[i].OfferDiscountPercentage = offer.DiscountPercentage
				cart.CartItems[i].OfferName = offer.OfferName
				cart.CartItems[i].IsOfferApplied = offer.IsOfferApplied
				cart.CartItems[i].SalePrice = offer.DiscountedPrice * float64(newQnty)

				if err := tx.Save(&cart.CartItems[i]).Error; err != nil {
					pkg.Log.Error("Failed to update existing cart item",
						zap.Uint("cart_id", cart.ID),
						zap.Uint("product_id", productID),
						zap.Uint("variant_id", variant.ID),
						zap.Error(err))
					return err
				}
				pkg.Log.Info("Updated existing cart item",
					zap.Uint("cart_id", cart.ID),
					zap.Uint("product_id", productID),
					zap.Uint("variant_id", variant.ID),
					zap.Uint("new_quantity", newQnty))
				return updateCartTotal(&cart, tx)
			}
		}

		// Create new cart item
		item := userModels.CartItem{
			CartID:                  cart.ID,
			ProductID:               productID,
			VariantsID:              variant.ID,
			Quantity:                req.Quantity,
			Price:                   product.Price + variantExtraPrice,
			DiscountedPrice:         offer.DiscountedPrice,
			RegularPrice:            product.Price + variantExtraPrice,
			OfferDiscountPercentage: offer.DiscountPercentage,
			OfferName:               offer.OfferName,
			IsOfferApplied:          offer.IsOfferApplied,
			SalePrice:               offer.DiscountedPrice * float64(req.Quantity),
		}

		if err := tx.Create(&item).Error; err != nil {
			pkg.Log.Error("Failed to create new cart item",
				zap.Uint("cart_id", cart.ID),
				zap.Uint("product_id", productID),
				zap.Uint("variant_id", variant.ID),
				zap.Error(err))
			return err
		}

		pkg.Log.Info("Created new cart item",
			zap.Uint("cart_id", cart.ID),
			zap.Uint("product_id", productID),
			zap.Uint("variant_id", variant.ID),
			zap.Uint("quantity", req.Quantity))

		if err := tx.Preload("CartItems").First(&cart, cart.ID).Error; err != nil {
			pkg.Log.Error("Failed to fetch cart after adding item",
				zap.Uint("cart_id", cart.ID),
				zap.Error(err))
			return err
		}

		return updateCartTotal(&cart, tx)
	})

	if err != nil {
		pkg.Log.Error("AddToCart transaction failed",
			zap.Any("user_id", userID),
			zap.Error(err))
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add to cart", "details": err.Error()})
		}
		return
	}

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).Preload("CartItems").First(&cart).Error; err != nil {
		pkg.Log.Error("Failed to fetch updated cart",
			zap.Any("user_id", userID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated cart"})
		return
	}

	pkg.Log.Info("Successfully added to cart",
		zap.Any("user_id", userID),
		zap.Uint("cart_id", cart.ID),
		zap.Int("item_count", len(cart.CartItems)))
	c.JSON(http.StatusOK, gin.H{
		"message": "Product added to cart successfully",
		"cart":    cart,
	})
}

func UpdateQuantity(c *gin.Context) {
	userID, _ := c.Get("id")
	var req RequestCartItem
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Log.Error("Invalid request payload for UpdateQuantity",
			zap.Any("user_id", userID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	pkg.Log.Debug("UpdateQuantity request",
		zap.Any("user_id", userID),
		zap.Uint("product_id", req.ProductID),
		zap.Uint("variant_id", req.VariantID),
		zap.Uint("quantity", req.Quantity))

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var cart userModels.Cart
		if err := tx.Where("user_id = ?", userID).Preload("CartItems").First(&cart).Error; err != nil {
			pkg.Log.Error("Failed to fetch cart",
				zap.Any("user_id", userID),
				zap.Error(err))
			return err
		}

		var variant adminModels.Variants
		if err := tx.Where("deleted_at IS NULL").First(&variant, req.VariantID).Error; err != nil {
			pkg.Log.Warn("Invalid or missing variant",
				zap.Uint("variant_id", req.VariantID),
				zap.Error(err))
			return gin.Error{Err: err, Meta: gin.H{"error": "Invalid or missing variant"}}
		}

		var product adminModels.Product
		if err := tx.Preload("Variants").Preload("Offers").First(&product, req.ProductID).Error; err != nil {
			pkg.Log.Error("Product not found",
				zap.Uint("product_id", req.ProductID),
				zap.Error(err))
			return gin.Error{Err: err, Meta: gin.H{"error": "Product not found"}}
		}

		if !product.IsListed {
			pkg.Log.Warn("Product is not available",
				zap.Uint("product_id", req.ProductID))
			return gin.Error{Meta: gin.H{"error": "Product is not available"}}
		}

		if req.Quantity > variant.Stock {
			pkg.Log.Warn("Quantity exceeds available stock",
				zap.Uint("variant_id", req.VariantID),
				zap.Uint("requested_quantity", req.Quantity),
				zap.Int("available_stock", int(variant.Stock)))
			return gin.Error{Meta: gin.H{
				"error": "Quantity exceeds available stock",
			}}
		}

		if req.Quantity > MAX_QUANTITY_PER_PRODUCT {
			pkg.Log.Warn("Quantity exceeds maximum limit",
				zap.Uint("variant_id", req.VariantID),
				zap.Uint("requested_quantity", req.Quantity),
				zap.Int("max_quantity", MAX_QUANTITY_PER_PRODUCT))
			return gin.Error{Meta: gin.H{
				"error": "Maximum limit is 5 items per product",
			}}
		}

		variantExtraPrice := variant.ExtraPrice
		offer := helper.GetBestOfferForProduct(&product, variantExtraPrice)

		for i, item := range cart.CartItems {
			if item.ProductID == req.ProductID && item.VariantsID == req.VariantID {
				if req.Quantity == 0 {
					if err := tx.Delete(&cart.CartItems[i]).Error; err != nil {
						pkg.Log.Error("Failed to delete cart item",
							zap.Uint("cart_id", cart.ID),
							zap.Uint("product_id", req.ProductID),
							zap.Uint("variant_id", req.VariantID),
							zap.Error(err))
						return err
					}
					pkg.Log.Info("Removed cart item due to zero quantity",
						zap.Uint("cart_id", cart.ID),
						zap.Uint("product_id", req.ProductID),
						zap.Uint("variant_id", req.VariantID))
				} else {
					cart.CartItems[i].Quantity = req.Quantity
					cart.CartItems[i].Price = product.Price + variantExtraPrice
					cart.CartItems[i].DiscountedPrice = offer.DiscountedPrice
					cart.CartItems[i].RegularPrice = product.Price + variantExtraPrice
					cart.CartItems[i].OfferDiscountPercentage = offer.DiscountPercentage
					cart.CartItems[i].OfferName = offer.OfferName
					cart.CartItems[i].IsOfferApplied = offer.IsOfferApplied
					cart.CartItems[i].SalePrice = (offer.DiscountedPrice) * float64(req.Quantity)
					if err := tx.Save(&cart.CartItems[i]).Error; err != nil {
						pkg.Log.Error("Failed to update cart item quantity",
							zap.Uint("cart_id", cart.ID),
							zap.Uint("product_id", req.ProductID),
							zap.Uint("variant_id", req.VariantID),
							zap.Error(err))
						return err
					}
					pkg.Log.Info("Updated cart item quantity",
						zap.Uint("cart_id", cart.ID),
						zap.Uint("product_id", req.ProductID),
						zap.Uint("variant_id", req.VariantID),
						zap.Uint("new_quantity", req.Quantity))
				}
				return updateCartTotal(&cart, tx)
			}
		}
		pkg.Log.Warn("Item not found in cart",
			zap.Any("user_id", userID),
			zap.Uint("product_id", req.ProductID),
			zap.Uint("variant_id", req.VariantID))
		return gin.Error{Meta: gin.H{"error": "Item not found in cart"}}
	})
	if err != nil {
		pkg.Log.Error("UpdateQuantity transaction failed",
			zap.Any("user_id", userID),
			zap.Error(err))
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update quantity"})
		}
		return
	}

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).Preload("CartItems").First(&cart).Error; err != nil {
		pkg.Log.Error("Failed to fetch updated cart",
			zap.Any("user_id", userID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated cart"})
		return
	}

	pkg.Log.Info("Successfully updated cart quantity",
		zap.Any("user_id", userID),
		zap.Uint("cart_id", cart.ID),
		zap.Uint("product_id", req.ProductID),
		zap.Uint("variant_id", req.VariantID),
		zap.Uint("quantity", req.Quantity))
	c.JSON(http.StatusOK, cart)
}

type varitReq struct {
	ProductID uint `json:"product_id"`
	VariantID uint `json:"variant_id"`
}

func RemoveFromCart(c *gin.Context) {
	userID, _ := c.Get("id")
	var req varitReq

	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Log.Error("Invalid request payload for RemoveFromCart",
			zap.Any("user_id", userID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	pkg.Log.Debug("RemoveFromCart request",
		zap.Any("user_id", userID),
		zap.Uint("product_id", req.ProductID),
		zap.Uint("variant_id", req.VariantID))

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var cart userModels.Cart
		if err := tx.Where("user_id = ?", userID).Preload("CartItems").First(&cart).Error; err != nil {
			pkg.Log.Error("Failed to fetch cart",
				zap.Any("user_id", userID),
				zap.Error(err))
			return err
		}

		for i, item := range cart.CartItems {
			if item.ProductID == req.ProductID && item.VariantsID == req.VariantID {
				if err := tx.Delete(&cart.CartItems[i]).Error; err != nil {
					pkg.Log.Error("Failed to delete cart item",
						zap.Uint("cart_id", cart.ID),
						zap.Uint("product_id", req.ProductID),
						zap.Uint("variant_id", req.VariantID),
						zap.Error(err))
					return err
				}
				pkg.Log.Info("Removed cart item",
					zap.Uint("cart_id", cart.ID),
					zap.Uint("product_id", req.ProductID),
					zap.Uint("variant_id", req.VariantID))
				return updateCartTotal(&cart, tx)
			}
		}
		pkg.Log.Warn("Item not found in cart",
			zap.Any("user_id", userID),
			zap.Uint("product_id", req.ProductID),
			zap.Uint("variant_id", req.VariantID))
		return gin.Error{Meta: gin.H{"error": "Item not found in cart"}}
	})
	if err != nil {
		pkg.Log.Error("RemoveFromCart transaction failed",
			zap.Any("user_id", userID),
			zap.Error(err))
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove item"})
		}
		return
	}
	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).Preload("CartItems").First(&cart).Error; err != nil {
		pkg.Log.Error("Failed to fetch updated cart",
			zap.Any("user_id", userID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated cart"})
		return
	}

	pkg.Log.Info("Successfully removed item from cart",
		zap.Any("user_id", userID),
		zap.Uint("cart_id", cart.ID),
		zap.Uint("product_id", req.ProductID),
		zap.Uint("variant_id", req.VariantID))
	c.JSON(http.StatusOK, cart)
}

func updateCartTotal(cart *userModels.Cart, tx *gorm.DB) error {
	var total float64
	for _, item := range cart.CartItems {
		var product adminModels.Product
		if err := tx.Preload("Variants").Preload("Offers").First(&product, item.ProductID).Error; err != nil {
			pkg.Log.Warn("Product not found in updateCartTotal",
				zap.Uint("product_id", item.ProductID),
				zap.Error(err))
			continue
		}
		if !product.IsListed {
			pkg.Log.Warn("Product unlisted in updateCartTotal",
				zap.Uint("product_id", item.ProductID))
			continue
		}

		var variant adminModels.Variants
		if err := tx.Where("deleted_at IS NULL").First(&variant, item.VariantsID).Error; err != nil {
			pkg.Log.Warn("Invalid or missing variant in updateCartTotal",
				zap.Uint("variant_id", item.VariantsID),
				zap.Uint("product_id", item.ProductID),
				zap.Error(err))
			continue
		}

		variantExtraPrice := variant.ExtraPrice
		offer := helper.GetBestOfferForProduct(&product, variantExtraPrice)

		item.Price = product.Price + variantExtraPrice
		item.DiscountedPrice = offer.DiscountedPrice
		item.RegularPrice = product.Price + variantExtraPrice
		item.OfferDiscountPercentage = offer.DiscountPercentage
		item.OfferName = offer.OfferName
		item.IsOfferApplied = offer.IsOfferApplied
		item.SalePrice = (offer.DiscountedPrice) * float64(item.Quantity)

		if err := tx.Save(&item).Error; err != nil {
			pkg.Log.Error("Failed to update cart item in updateCartTotal",
				zap.Uint("cart_id", cart.ID),
				zap.Uint("product_id", item.ProductID),
				zap.Uint("variant_id", item.VariantsID),
				zap.Error(err))
			return err
		}

		if variant.Stock >= item.Quantity {
			total += item.SalePrice
		}
	}
	cart.TotalPrice = total
	if err := tx.Save(cart).Error; err != nil {
		pkg.Log.Error("Failed to save cart total",
			zap.Uint("cart_id", cart.ID),
			zap.Float64("total_price", total),
			zap.Error(err))
		return err
	}

	pkg.Log.Debug("Updated cart total",
		zap.Uint("cart_id", cart.ID),
		zap.Float64("total_price", total))
	return nil
}
