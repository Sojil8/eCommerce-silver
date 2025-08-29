package controllers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const MAX_QUANTITY_PER_PRODUCT = 5

func GetCart(c *gin.Context) {
	userID := helper.FetchUserID(c)
	var cart userModels.Cart

	err := database.DB.Where("user_id = ?", userID).Preload("CartItems.Product").Preload("CartItems.Variants").First(&cart).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			cart = userModels.Cart{UserID: userID}
			if err := database.DB.Create(&cart).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cart"})
				return
			}
		} else {
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
			log.Printf("Product not found for ProductID: %d, removing cart item", item.ProductID)
			database.DB.Delete(&item)
			invalidItems = append(invalidItems, fmt.Sprintf("Product %d", item.ProductID))
			continue
		}

		var category adminModels.Category
		if !product.IsListed ||
			database.DB.Where("category_name = ? AND status = ?", product.CategoryName, true).First(&category).Error != nil {
			invalidItems = append(invalidItems, fmt.Sprintf("Product %d (unlisted or invalid category)", item.ProductID))
			continue
		}

		var variant adminModels.Variants
		if err := database.DB.Where("deleted_at IS NULL").First(&variant, item.VariantsID).Error; err != nil || variant.ProductID != item.ProductID {
			log.Printf("Invalid or missing variant for VariantsID: %d, ProductID: %d, skipping cart item", item.VariantsID, item.ProductID)
			invalidItems = append(invalidItems, fmt.Sprintf("Product %d (Variant %d)", item.ProductID, item.VariantsID))
			continue
		}

		item.Product = product
		item.Variants = variant

		isInStock := variant.Stock > 0 && variant.Stock >= item.Quantity
		item.Product.InStock = isInStock

		log.Printf("CartItem: ProductID=%d, VariantID=%d, ProductName=%s, Stock=%d, Quantity=%d, InStock=%v",
			item.ProductID, item.VariantsID, product.ProductName, variant.Stock, item.Quantity, isInStock)

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart total"})
		return
	}

	log.Printf("Cart Summary: TotalItems=%d, InStockItems=%d, OutOfStockItems=%d",
		len(allCartItems), len(inStockCartItems), len(outOfStockCartItems))

	userr, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
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
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		cartCount = 0
	}

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
	VariantID  uint  `json:"variant_id"`
	Quantity   uint  `json:"quantity"`
}

func AddToCart(c *gin.Context) {
	userID, _ := c.Get("id")

	var req CartInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var productID uint

		if req.WishlistID != nil {
			var wishlist userModels.Wishlist
			if err := tx.First(&wishlist, *req.WishlistID).Error; err != nil {
				return gin.Error{Err: err, Meta: gin.H{"error": "Wishlist item not found"}}
			}
			if wishlist.UserID != userID.(uint) {
				return gin.Error{Meta: gin.H{"error": "Unauthorized access to wishlist item"}}
			}
			productID = wishlist.ProductID
			if err := tx.Where("user_id = ? AND id = ?", userID, *req.WishlistID).
				Delete(&userModels.Wishlist{}).Error; err != nil {
				return err
			}
		} else if req.ProductID != nil {
			productID = *req.ProductID
		} else {
			return gin.Error{Meta: gin.H{"error": "Either product_id or wishlist_id is required"}}
		}

		var product adminModels.Product
		if err := tx.Preload("Variants").Preload("Offers").First(&product, productID).Error; err != nil {
			return gin.Error{Err: err, Meta: gin.H{"error": "Product not found"}}
		}

		var category adminModels.Category
		if err := tx.Where("category_name = ?", product.CategoryName).First(&category).Error; err != nil || !category.Status {
			return gin.Error{Err: err, Meta: gin.H{"error": "Product category is not available"}}
		}

		if !product.IsListed {
			return gin.Error{Meta: gin.H{"error": "Product is not available"}}
		}

		var variant adminModels.Variants
		if err := tx.Where("deleted_at IS NULL").First(&variant, req.VariantID).Error; err != nil || productID != variant.ProductID {
			return gin.Error{Err: err, Meta: gin.H{"error": "Invalid or missing variant"}}
		}

		if variant.Stock < req.Quantity {
			return gin.Error{Meta: gin.H{"error": "Maximum quantity exceeded"}}
		}

		if req.Quantity > MAX_QUANTITY_PER_PRODUCT {
			return gin.Error{Meta: gin.H{"error": "Quantity exceeds maximum limit"}}
		}

		var cart userModels.Cart
		if err := tx.Where("user_id = ?", userID).Preload("CartItems").
			FirstOrCreate(&cart, userModels.Cart{UserID: userID.(uint)}).Error; err != nil {
			return err
		}

		variantExtraPrice := variant.ExtraPrice
		offer := helper.GetBestOfferForProduct(&product, variantExtraPrice)

		for i, item := range cart.CartItems {
			if item.ProductID == productID && item.VariantsID == req.VariantID {
				newQnty := item.Quantity + req.Quantity
				if newQnty > MAX_QUANTITY_PER_PRODUCT || newQnty > variant.Stock {
					return gin.Error{Meta: gin.H{"error": "Quantity limit exceeded"}}
				}

				cart.CartItems[i].Quantity = newQnty
				cart.CartItems[i].Price = product.Price + variantExtraPrice
				cart.CartItems[i].DiscountedPrice = offer.DiscountedPrice
				cart.CartItems[i].RegularPrice = product.Price
				cart.CartItems[i].OfferDiscountPercentage = offer.DiscountPercentage
				cart.CartItems[i].OfferName = offer.OfferName
				cart.CartItems[i].IsOfferApplied = offer.IsOfferApplied

				if err := tx.Save(&cart.CartItems[i]).Error; err != nil {
					return err
				}
				return updateCartTotal(&cart, tx)
			}
		}

		item := userModels.CartItem{
			CartID:                  cart.ID,
			ProductID:               productID,
			VariantsID:              req.VariantID,
			Quantity:                req.Quantity,
			Price:                   product.Price,
			DiscountedPrice:         offer.DiscountedPrice,
			RegularPrice:            product.Price + variantExtraPrice,
			OfferDiscountPercentage: offer.DiscountPercentage,
			OfferName:               offer.OfferName,
			IsOfferApplied:          offer.IsOfferApplied,
			SalePrice:               (offer.DiscountedPrice) * float64(req.Quantity),
		}

		if err := tx.Create(&item).Error; err != nil {
			return err
		}

		if err := tx.Preload("CartItems").First(&cart, cart.ID).Error; err != nil {
			return err
		}

		return updateCartTotal(&cart, tx)
	})

	if err != nil {
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add to cart", "details": err.Error()})
		}
		return
	}

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).Preload("CartItems").First(&cart).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated cart"})
		return
	}
	c.JSON(http.StatusOK, cart)
}

func UpdateQuantity(c *gin.Context) {
	userID, _ := c.Get("id")
	var req RequestCartItem
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var cart userModels.Cart
		if err := tx.Where("user_id = ?", userID).Preload("CartItems").First(&cart).Error; err != nil {
			return err
		}

		var variant adminModels.Variants
		if err := tx.Where("deleted_at IS NULL").First(&variant, req.VariantID).Error; err != nil {
			return gin.Error{Err: err, Meta: gin.H{"error": "Invalid or missing variant"}}
		}

		var product adminModels.Product
		if err := tx.Preload("Variants").Preload("Offers").First(&product, req.ProductID).Error; err != nil {
			return gin.Error{Err: err, Meta: gin.H{"error": "Product not found"}}
		}

		if !product.IsListed {
			return gin.Error{Meta: gin.H{"error": "Product is not available"}}
		}

		if req.Quantity > variant.Stock {
			return gin.Error{Meta: gin.H{
				"error": "Quantity exceeds available stock",
			}}
		}

		if req.Quantity > MAX_QUANTITY_PER_PRODUCT {
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
						return err
					}
				} else {
					cart.CartItems[i].Quantity = req.Quantity
					cart.CartItems[i].Price = product.Price + variantExtraPrice
					cart.CartItems[i].DiscountedPrice = offer.DiscountedPrice
					cart.CartItems[i].RegularPrice = product.Price + variantExtraPrice
					cart.CartItems[i].OfferDiscountPercentage = offer.DiscountPercentage
					cart.CartItems[i].OfferName = offer.OfferName
					cart.CartItems[i].IsOfferApplied = offer.IsOfferApplied
					cart.CartItems[i].SalePrice = (offer.DiscountedPrice + variantExtraPrice) * float64(req.Quantity)
					if err := tx.Save(&cart.CartItems[i]).Error; err != nil {
						return err
					}
				}
				return updateCartTotal(&cart, tx)
			}
		}
		return gin.Error{Meta: gin.H{"error": "Item not found in cart"}}
	})
	if err != nil {
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update quantity"})
		}
		return
	}

	var cart userModels.Cart
	database.DB.Where("user_id = ?", userID).Preload("CartItems").First(&cart)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var cart userModels.Cart
		if err := tx.Where("user_id = ?", userID).Preload("CartItems").First(&cart).Error; err != nil {
			return err
		}

		for i, item := range cart.CartItems {
			if item.ProductID == req.ProductID && item.VariantsID == req.VariantID {
				if err := tx.Delete(&cart.CartItems[i]).Error; err != nil {
					return err
				}
				return updateCartTotal(&cart, tx)
			}
		}
		return gin.Error{Meta: gin.H{"error": "Item not found in cart"}}
	})
	if err != nil {
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove item"})
		}
		return
	}
	var cart userModels.Cart
	database.DB.Where("user_id = ?", userID).Preload("CartItems").First(&cart)
	c.JSON(http.StatusOK, cart)
}

func updateCartTotal(cart *userModels.Cart, tx *gorm.DB) error {
	var total float64
	for _, item := range cart.CartItems {
		var product adminModels.Product
		if err := tx.Preload("Variants").Preload("Offers").First(&product, item.ProductID).Error; err != nil {
			log.Printf("Product not found for ProductID: %d, removing cart item", item.ProductID)
			continue
		}
		if !product.IsListed {
			log.Printf("Product unlisted for ProductID: %d, removing cart item", item.ProductID)
			continue
		}

		var variant adminModels.Variants
		if err := tx.Where("deleted_at IS NULL").First(&variant, item.VariantsID).Error; err != nil {
			log.Printf("Invalid or missing variant for VariantsID: %d, ProductID: %d, skipping cart item", item.VariantsID, item.ProductID)
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
			return err
		}

		if variant.Stock >= item.Quantity {
			total += item.SalePrice
		}
	}
	cart.TotalPrice = total
	return tx.Save(cart).Error
}
