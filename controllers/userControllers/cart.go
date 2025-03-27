package controllers

import (
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const MAX_QUANTITY_PER_PRODUCT = 10

func GetCart(c *gin.Context) {
    userID, _ := c.Get("id")
    var cart userModels.Cart
    err := database.DB.Where("user_id = ?", userID).Preload("CartItems.Product").Preload("CartItems.Variants").First(&cart).Error
    if err != nil {
        if err == gorm.ErrRecordNotFound {
            cart = userModels.Cart{UserID: userID.(uint)}
            if err := database.DB.Create(&cart).Error; err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cart"})
                return
            }
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cart"})
            return
        }
    }
    userName, exists := c.Get("user_name")
    if !exists {
        userName = "Guest"
    }
    c.HTML(http.StatusOK, "cart.html", gin.H{
        "CartItems":     cart.CartItems,
        "TotalPrice":    cart.TotalPrice,
        "UserName":      userName,
        "CartItemCount": len(cart.CartItems),
    })
}
type RequestCartItem struct {
	ProductID uint `json:"product_id"`
	VariantID uint `json:"variant_id"`
	Quantity  uint `json:"quantity"`
}

func AddToCart(c *gin.Context) {
	userID, _ := c.Get("id")
	var req RequestCartItem
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var product adminModels.Product
		if err := tx.Preload("Variants").First(&product, req.ProductID).Error; err != nil {
			return err
		}

		var category adminModels.Category
		if err := tx.Where("category_name = ?", product.CategoryName).First(&category).Error; err != nil || !category.Status {
			return gin.Error{Err: err, Meta: gin.H{"error": "Product category is not available"}}
		}

		if !product.IsListed {
			return gin.Error{Meta: gin.H{"error": "Product is not available"}}
		}

		var variant adminModels.Variants
		if err := tx.First(&variant, req.VariantID).Error; err != nil || req.ProductID != variant.ProductID {
			return gin.Error{Err: err, Meta: gin.H{"error": "Invalid variant"}}
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

		for i, item := range cart.CartItems {
			if item.ProductID == req.ProductID && item.VariantsID == req.VariantID {
				newQnty := item.Quantity + req.Quantity
				if newQnty > MAX_QUANTITY_PER_PRODUCT || newQnty > variant.Stock {
					return gin.Error{Meta: gin.H{"error": "Quantity limit exceeded"}}
				}
				cart.CartItems[i].Quantity = newQnty
				if err := tx.Save(&cart.CartItems[i]).Error; err != nil {
					return err
				}
				updateCartTotal(&cart, tx)
				return nil
			}
		}

		item := userModels.CartItem{
			CartID:     cart.ID,
			ProductID:  req.ProductID,
			VariantsID: req.VariantID,
			Quantity:   req.Quantity,
			Price:      product.Price + variant.ExtraPrice,
		}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}

		if err := tx.Where("user_id = ? AND product_id = ?", userID, req.ProductID).
			Delete(&userModels.Wishlist{}).Error; err != nil {
			return err
		}
		updateCartTotal(&cart, tx)
		return nil
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
	database.DB.Where("user_id = ?", userID).Preload("CartItems").First(&cart)
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

		var variants adminModels.Variants
		if err := tx.First(&variants, req.VariantID).Error; err != nil {
			return err
		}

		for i, item := range cart.CartItems {
			if item.ProductID == req.ProductID && item.VariantsID == req.VariantID {
				if req.Quantity > variants.Stock || req.Quantity > MAX_QUANTITY_PER_PRODUCT {
					return gin.Error{Meta: gin.H{
						"error": "Quantity exceeds available stock or limit",
					}}
				}
				if req.Quantity == 0 {
					if err := tx.Delete(&cart.CartItems[i]).Error; err != nil {
						return err
					}
				} else {
					cart.CartItems[i].Quantity = req.Quantity
					if err := tx.Save(&cart.CartItems[i]).Error; err != nil {
						return err
					}
				}
				updateCartTotal(&cart, tx)
				return nil
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
				updateCartTotal(&cart, tx)
				return nil
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
		if err := tx.First(&product, item.ProductID).Error; err != nil {
			return err
		}
		total += item.Price * float64(item.Quantity)
	}
	cart.TotalPrice = total
	return tx.Save(cart).Error
}
