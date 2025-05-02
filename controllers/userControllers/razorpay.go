package controllers

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PaymentVerification struct {
	RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
	RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
	RazorpaySignature string `json:"razorpay_signature" binding:"required"`
}

func VerifyPayment(c *gin.Context) {
	userID, _ := c.Get("id")
	var orderIdUnique string

	var req PaymentVerification
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":   "error",
			"message":  "Invalid payment data",
			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape("Invalid payment data")),
		})
		return
	}

	if !helper.VerifyPaymentSignature(req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":   "error",
			"message":  "Invalid payment signature",
			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape("Invalid payment signature")),
		})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var razorpayPayment adminModels.PaymentDetails
		if err := tx.Where("razorpay_order_id = ? AND user_id = ?", req.RazorpayOrderID, userID).First(&razorpayPayment).Error; err != nil {
			return fmt.Errorf("payment record not found: %v", err)
		}

		var cart userModels.Cart
		if err := tx.Where("user_id = ?", userID).Preload("CartItems.Product").
			Preload("CartItems.Variants").First(&cart).Error; err != nil {
			return fmt.Errorf("cart not found: %v", err)
		}

		var validateCartItem []userModels.CartItem
		for _, item := range cart.CartItems {
			var category adminModels.Category
			if item.Product.IsListed && tx.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error == nil {
				validateCartItem = append(validateCartItem, item)
			}
		}

		if len(validateCartItem) == 0 {
			return fmt.Errorf("no valid cart items found")
		}

		var address userModels.Address
		if err := tx.Where("user_id = ? AND is_default = ?", userID, true).First(&address).Error; err != nil {
			return fmt.Errorf("no default address found: %v", err)
		}

		finalPrice := cart.TotalPrice + shipping
		var discount float64
		var coupon adminModels.Coupons

		if cart.CouponID != 0 {
			if err := tx.First(&coupon, cart.CouponID).Error; err == nil {
				if coupon.IsActive && coupon.ExpiryDate.After(time.Now()) &&
					coupon.UsedCount < coupon.UsageLimit &&
					cart.TotalPrice >= coupon.MinPurchaseAmount &&
					(coupon.MaxPurchaseAmount == 0 || cart.TotalPrice <= coupon.MaxPurchaseAmount) {
					discount = coupon.DiscountPercentage
					finalPrice -= discount
				} else {
					cart.CouponID = 0
					if err := tx.Save(&cart).Error; err != nil {
						return fmt.Errorf("failed to reset coupon: %v", err)
					}
				}
			} else {
				cart.CouponID = 0
				if err := tx.Save(&cart).Error; err != nil {
					return fmt.Errorf("failed to reset coupon: %v", err)
				}
			}
		}

		// Validate minimum order amount
		if finalPrice < minimumOrderAmount {
			return fmt.Errorf("order amount too low, must be at least ₹%.2f", minimumOrderAmount)
		}

		orderID := generateOrderID()
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
			TotalPrice:    finalPrice,
			Subtotal:      cart.TotalPrice,
			Discount:      discount,
			CouponID:      cart.CouponID,
			Status:        "Confirmed",
			PaymentMethod: "ONLINE",
			OrderDate:     time.Now(),
		}

		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("failed to create order: %v", err)
		}

		for _, item := range validateCartItem {
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

			var Variants adminModels.Variants
			if err := tx.First(&Variants, item.VariantsID).Error; err != nil {
				return err
			}
			if Variants.Stock < item.Quantity {
				return fmt.Errorf("insufficient stock for variant ID %d", item.VariantsID)
			}

			Variants.Stock -= item.Quantity
			if Variants.Stock == 0 {
				var product adminModels.Product
				if err := tx.First(&product, item.ProductID).Error; err != nil {
					return err
				}
				product.InStock = false
				if err := tx.Save(&product).Error; err != nil {
					return err
				}
			}
			if err := tx.Save(&Variants).Error; err != nil {
				return err
			}
		}

		if cart.CouponID != 0 {
			coupon.UsedCount++
			if err := tx.Save(&coupon).Error; err != nil {
				return fmt.Errorf("failed to update coupon usage: %v", err)
			}
		}

		razorpayPayment.OrderID = order.ID
		razorpayPayment.RazorpayPaymentID = req.RazorpayPaymentID
		razorpayPayment.RazorpaySignature = req.RazorpaySignature
		razorpayPayment.Status = "Success"
		razorpayPayment.Attempts++

		if err := tx.Save(&razorpayPayment).Error; err != nil {
			return fmt.Errorf("failed to update payment status: %v", err)
		}

		if err := tx.Where("cart_id = ?", cart.ID).Delete(&userModels.CartItem{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&cart).Error; err != nil {
			return err
		}

		orderIdUnique = order.OrderIdUnique
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":   "error",
			"message":  err.Error(),
			"redirect": fmt.Sprintf("/order/failure?error=%s&order_id=%s", url.QueryEscape(err.Error()), orderIdUnique),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "Payment verified successfully",
		"redirect": fmt.Sprintf("/order/success?order_id=%s", orderIdUnique),
	})
}
