package controllers

import (
	"fmt"
	"log"
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
	// 1. Extract and validate user ID
	userID, exists := c.Get("id")
	if !exists {
		log.Printf("No user ID found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":   "error",
			"message":  "User not authenticated",
			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape("User not authenticated")),
		})
		return
	}
	uid, ok := userID.(uint)
	if !ok {
		log.Printf("Invalid user ID type")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":   "error",
			"message":  "Internal server error",
			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape("Invalid user ID type")),
		})
		return
	}

	// 2. Bind JSON request
	var req PaymentVerification
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":   "error",
			"message":  "Invalid payment data",
			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape("Invalid payment data")),
		})
		return
	}

	// 3. Verify payment signature
	if !helper.VerifyPaymentSignature(req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature) {
		log.Printf("Invalid payment signature for Razorpay order ID: %s", req.RazorpayOrderID)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":   "error",
			"message":  "Invalid payment signature",
			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape("Invalid payment signature")),
		})
		return
	}

	// 4. Initialize variables
	var orderIdUnique string
	var cart userModels.Cart
	var validCartItems []userModels.CartItem
	var address userModels.Address
	var finalPrice float64
	var couponDiscount float64
	var offerDiscount float64
	var coupon adminModels.Coupons

	// 5. Transaction for order creation
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Fetch payment details
		var paymentDetails adminModels.PaymentDetails
		if err := tx.Where("razorpay_order_id = ? AND user_id = ?", req.RazorpayOrderID, uid).First(&paymentDetails).Error; err != nil {
			return fmt.Errorf("payment record not found: %v", err)
		}

		// Validate cart
		if err := tx.Where("user_id = ?", uid).
			Preload("CartItems.Product").
			Preload("CartItems.Variants").
			First(&cart).Error; err != nil {
			return fmt.Errorf("cart not found: %v", err)
		}

		// Validate cart items and calculate offer discounts
		totalPrice := 0.0
		for _, item := range cart.CartItems {
			var category adminModels.Category
			var variant adminModels.Variants

			isInStock := item.Product.IsListed && item.Product.InStock
			hasVariantStock := false
			if err := tx.Where("id = ? AND deleted_at IS NULL", item.VariantsID).First(&variant).Error; err == nil {
				hasVariantStock = variant.Stock >= item.Quantity
			}

			if isInStock && hasVariantStock &&
				tx.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error == nil {
				offerDetails := helper.GetBestOfferForProduct(&item.Product, item.Variants.ExtraPrice)
				item.Price = offerDetails.OriginalPrice
				item.DiscountedPrice = offerDetails.DiscountedPrice
				item.OriginalPrice = offerDetails.OriginalPrice
				item.DiscountPercentage = offerDetails.DiscountPercentage
				item.OfferName = offerDetails.OfferName
				item.IsOfferApplied = offerDetails.IsOfferApplied
				item.ItemTotal = offerDetails.DiscountedPrice * float64(item.Quantity)
				totalPrice += item.ItemTotal
				offerDiscount += (offerDetails.OriginalPrice - offerDetails.DiscountedPrice) * float64(item.Quantity)
				validCartItems = append(validCartItems, item)
			} else {
				log.Printf("Skipping item: product_id=%d, variant_id=%d, product_listed=%v, product_in_stock=%v, variant_stock=%d, quantity=%d",
					item.ProductID, item.VariantsID, item.Product.IsListed, isInStock, variant.Stock, item.Quantity)
			}
		}

		if len(validCartItems) == 0 {
			return fmt.Errorf("no valid items in cart with sufficient stock")
		}

		// Update cart total
		cart.TotalPrice = totalPrice
		if err := tx.Save(&cart).Error; err != nil {
			return fmt.Errorf("failed to save cart: %v", err)
		}


		// Calculate final price and apply coupon
		finalPrice = cart.TotalPrice + shipping 
		couponCode := ""
		if cart.CouponID != 0 {
			if err := tx.First(&coupon, cart.CouponID).Error; err == nil {
				if coupon.IsActive &&
					coupon.ExpiryDate.After(time.Now()) &&
					coupon.UsedCount < coupon.UsageLimit &&
					cart.TotalPrice >= coupon.MinPurchaseAmount {
					couponDiscount = cart.TotalPrice * (coupon.DiscountPercentage / 100)
					finalPrice -= couponDiscount
					couponCode = coupon.CouponCode
				} else {
					log.Printf("Coupon %s invalid: active=%v, expired=%v, used=%d/%d, min=%.2f, cartTotal=%.2f",
						coupon.CouponCode, coupon.IsActive, coupon.ExpiryDate.Before(time.Now()),
						coupon.UsedCount, coupon.UsageLimit, coupon.MinPurchaseAmount, cart.TotalPrice)
					cart.CouponID = 0
					if err := tx.Save(&cart).Error; err != nil {
						return fmt.Errorf("failed to reset coupon: %v", err)
					}
				}
			} else {
				log.Printf("Coupon ID %v not found: %v", cart.CouponID, err)
				cart.CouponID = 0
				if err := tx.Save(&cart).Error; err != nil {
					return fmt.Errorf("failed to reset coupon: %v", err)
				}
			}
		}

		// Validate minimum order amount
		if finalPrice < minimumOrderAmount {
			return fmt.Errorf("order amount too low: %.2f < %.2f", finalPrice, minimumOrderAmount)
		}

		

		// Create shipping address
		orderID := generateOrderID()
		shippingAdd := adminModels.ShippingAddress{
			OrderID:        orderID,
			UserID:         uid,
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

		// Create order
		order := userModels.Orders{
			UserID:                uid,
			OrderIdUnique:         orderID,
			// AddressID:             paymentDetails.AddressID,
			ShippingAddress:       shippingAdd,
			TotalPrice:            finalPrice,
			Subtotal:              cart.TotalPrice,
			CouponDiscount:        couponDiscount,
			OfferDiscount:         offerDiscount,
			CouponID:              cart.CouponID,
			CouponCode:            couponCode,
			Status:                "Confirmed",
			PaymentMethod:         "ONLINE",
			PaymentStatus:         "Paid",
			OrderDate:             time.Now(),
			ShippingCost:          shipping,
			TrackingNumber:        "",
			EstimatedDeliveryDate: time.Now().AddDate(0, 0, 7),
			CancellationStatus:    "None",
		}
		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("failed to create order: %v", err)
		}

		// Create order items and update stock
		for _, item := range validCartItems {
			// variantAttributes := fmt.Sprintf(`{"color": "%s", "size": "%s"}`, item.Variants.Color, item.Variants.Size)
			orderItem := userModels.OrderItem{
				OrderID:           order.ID,
				ProductID:         item.ProductID,
				VariantsID:        item.VariantsID,
				Quantity:          item.Quantity,
				UnitPrice:         item.DiscountedPrice,
				ItemTotal:         item.DiscountedPrice * float64(item.Quantity),
				DiscountAmount:    (item.OriginalPrice - item.DiscountedPrice) * float64(item.Quantity),
				OfferName:         item.OfferName,
				Status:            "Active",
				ReturnStatus:      "None",
				// VariantAttributes: variantAttributes,
			}
			if err := tx.Create(&orderItem).Error; err != nil {
				return fmt.Errorf("failed to create order item: %v", err)
			}

			var variant adminModels.Variants
			if err := tx.First(&variant, item.VariantsID).Error; err != nil {
				return fmt.Errorf("variant not found: %v", err)
			}
			if variant.Stock < item.Quantity {
				return fmt.Errorf("insufficient stock for variant ID %d", item.VariantsID)
			}
			variant.Stock -= item.Quantity
			if variant.Stock == 0 {
				var product adminModels.Product
				if err := tx.First(&product, item.ProductID).Error; err != nil {
					return fmt.Errorf("product not found: %v", err)
				}
				product.InStock = false
				if err := tx.Save(&product).Error; err != nil {
					return fmt.Errorf("failed to update product stock: %v", err)
				}
			}
			if err := tx.Save(&variant).Error; err != nil {
				return fmt.Errorf("failed to update variant stock: %v", err)
			}
		}

		// Update coupon usage
		if cart.CouponID != 0 {
			coupon.UsedCount++
			if err := tx.Save(&coupon).Error; err != nil {
				return fmt.Errorf("failed to update coupon usage: %v", err)
			}
		}

		// Update payment details
		paymentDetails.OrderID = order.ID
		paymentDetails.RazorpayPaymentID = req.RazorpayPaymentID
		paymentDetails.RazorpaySignature = req.RazorpaySignature
		paymentDetails.Status = "Paid"
		paymentDetails.Attempts++
		if err := tx.Save(&paymentDetails).Error; err != nil {
			return fmt.Errorf("failed to update payment status: %v", err)
		}

		// Delete cart
		if err := tx.Where("cart_id = ?", cart.ID).Delete(&userModels.CartItem{}).Error; err != nil {
			return fmt.Errorf("failed to delete cart items: %v", err)
		}
		if err := tx.Delete(&cart).Error; err != nil {
			return fmt.Errorf("failed to delete cart: %v", err)
		}

		orderIdUnique = order.OrderIdUnique
		return nil
	})

	// 6. Handle transaction error
	if err != nil {
		log.Printf("Transaction error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":   "error",
			"message":  "Failed to verify payment",
			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape(err.Error())),
		})
		return
	}

	// 7. Prepare response
	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "Payment verified successfully",
		"order_id": orderIdUnique,
		"redirect": fmt.Sprintf("/order/success?order_id=%s", orderIdUnique),
	})
}
