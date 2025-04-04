package controllers

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const shipping = 10.0

func ShowCheckout(c *gin.Context) {
	userID, _ := c.Get("id")
	userName, _ := c.Get("user_name")

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).
		Preload("CartItems.Product").
		Preload("CartItems.Variants").
		First(&cart).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Cart not found", "Please add items to cart", "")
		return
	}

	var validCartItems []userModels.CartItem
	var invalidProductFound bool
	for _, item := range cart.CartItems {
		var category adminModels.Category
		if item.Product.IsListed &&
			database.DB.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error == nil {
			validCartItems = append(validCartItems, item)
		} else {
			invalidProductFound = true
		}
	}

	if len(validCartItems) == 0 {
		helper.ResponseWithErr(c, http.StatusBadRequest, "No valid products in cart",
			"Some products in your cart are no longer available", "")
		return
	}

	if invalidProductFound {
		totalPrice := 0.0
		for _, item := range validCartItems {
			totalPrice += item.Price * float64(item.Quantity)
		}
		cart.TotalPrice = totalPrice
		cart.CartItems = validCartItems
	}

	var addresses []userModels.Address
	if err := database.DB.Where("user_id = ?", userID).Find(&addresses).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to load addresses", "Database error", "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to load user details", "Database error", "")
		return
	}

	finalPrice := cart.TotalPrice + shipping

	c.HTML(http.StatusOK, "checkout.html", gin.H{
		"title":      "Checkout",
		"Cart":       cart,
		"Addresses":  addresses,
		"Shipping":   shipping,
		"FinalPrice": finalPrice,
		"UserName":   userName,
		"UserEmail":  user.Email,
		"UserPhone":  user.Phone,
	})
}

func SetDefaultAddress(c *gin.Context) {
	userID, _ := c.Get("id")
	addressID := c.Param("address_id")

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&userModels.Address{}).
			Where("user_id = ?", userID).
			Update("is_default", false).Error; err != nil {
			return err
		}
		var address userModels.Address
		if err := tx.Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error; err != nil {
			return err
		}
		address.IsDefault = true
		return tx.Save(&address).Error
	})

	if err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to set default address", "Database error", "")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Default address updated"})
}

type PlaceOrderRequest struct {
    AddressID uint `json:"address_id" binding:"required"`
}

func PlaceOrder(c *gin.Context) {
	userID, _ := c.Get("id")

	var req PlaceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", "Please select an address", "")
		return
	}

	orderID := generateOrderID()

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var cart userModels.Cart
		if err := tx.Where("user_id = ?", userID).
			Preload("CartItems.Product").
			Preload("CartItems.Variants").
			First(&cart).Error; err != nil {
			return err
		}

		var validCartItems []userModels.CartItem
		for _, item := range cart.CartItems {
			var category adminModels.Category
			if item.Product.IsListed &&
				tx.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error == nil {
				validCartItems = append(validCartItems, item)
			}
		}

		var address userModels.Address
		if err := tx.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address).Error; err != nil {
			return err
		}

		shippingAdd := adminModels.ShippingAddress{
			OrderID:        orderID,
			UserID:         address.UserID,
			Name:           address.Name,
			City:           address.City,
			Landmark:       address.Landmark,
			State:          address.State,
			Pincode:        address.Pincode,
			AddressType:    address.AddressType,
			Phone:          address.Phone,
			AlternatePhone: address.AlternatePhone,
			TrackingStatus: "Pending",
		}
		if err := tx.Create(&shippingAdd).Error; err != nil {
			return fmt.Errorf("failed to create shipping address: %v", err)
		}

		order := userModels.Orders{
			UserID:        userID.(uint),
			OrderIdUnique: orderID,
			AddressID:     address.ID,
			TotalPrice:    cart.TotalPrice + shipping,
			Status:        "Pending",
			PaymentMethod: "Cash On Delivery",
			OrderDate:     time.Now(),
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}

		for _, item := range cart.CartItems {
			orderItem := userModels.OrderItem{
				OrderID:    order.ID,
				ProductID:  item.ProductID,
				VariantsID: item.VariantsID,
				Quantity:   item.Quantity,
				Price:      item.Price,
				Status:     "Active",
			}
			if err := tx.Create(&orderItem).Error; err != nil {
				return err
			}

			var variant adminModels.Variants
			if err := tx.First(&variant, item.VariantsID).Error; err != nil {
				return err
			}
			if variant.Stock < item.Quantity {
				return fmt.Errorf("insufficient stock for variant ID %d", item.VariantsID)
			}

			variant.Stock -= item.Quantity
			if variant.Stock ==0{
				var product adminModels.Product
				if err:=tx.First(&product,item.ProductID).Error;err!=nil{
					return err
				}
				product.InStock = false
				if err:=tx.Save(&product).Error;err!=nil{
					return err
				}
			}

			if err := tx.Save(&variant).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("cart_id = ?", cart.ID).Delete(&userModels.CartItem{}).Error; err != nil {
			return err
		}

		return tx.Delete(&cart).Error
	})

	if err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to place order", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "Order placed successfully",
		"order_id": orderID,
	})
}

func generateOrderID() string {
	timestamp := time.Now().Format("20060102")
	randomNum := rand.Intn(10000)
	return fmt.Sprintf("ORD-%s-%04d", timestamp, randomNum)
}

func ShowOrderSuccess(c *gin.Context) {
	userID, _ := c.Get("id")
	orderID := c.Query("order_id")
	var order userModels.Orders
	if err := database.DB.Where("order_id_unique  = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to load order", "Database error", "")
		return
	}

	c.HTML(http.StatusOK, "orderSuccess.html", gin.H{
		"title":   "Order Successful",
		"OrderID": order.OrderIdUnique,
	})
}
