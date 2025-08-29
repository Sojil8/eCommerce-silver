package controllers

// import (
// 	"fmt"
// 	"log"
// 	"net/http"
// 	"net/url"
// 	"os"
// 	"time"

// 	"github.com/Sojil8/eCommerce-silver/database"
// 	"github.com/Sojil8/eCommerce-silver/models/adminModels"
// 	"github.com/Sojil8/eCommerce-silver/models/userModels"
// 	"github.com/Sojil8/eCommerce-silver/utils/helper"
// 	"github.com/gin-gonic/gin"
// 	"github.com/razorpay/razorpay-go"
// 	"gorm.io/gorm"
// )

// type PaymentRequest1 struct {
// 	AddressID     uint   `json:"address_id" binding:"required"`
// 	PaymentMethod string `json:"payment_method" binding:"required"`
// }

// func PlaceOrder1(c *gin.Context) {
// 	userID, exists := c.Get("id")
// 	if !exists {
// 		log.Printf("No user ID found in context")
// 		helper.ResponseWithErr(c, http.StatusUnauthorized, "User not authenticated", "No user ID found in context", "")
// 		return
// 	}
// 	uid, ok := userID.(uint)
// 	if !ok {
// 		log.Printf("Invalid user ID type")
// 		helper.ResponseWithErr(c, http.StatusInternalServerError, "Internal server error", "Invalid user ID type", "")
// 		return
// 	}

// 	var req PaymentRequest
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		log.Printf("Error binding JSON: %v", err)
// 		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
// 		return
// 	}

// 	log.Printf("PlaceOrder request for user %v: address_id=%d, payment_method=%s", uid, req.AddressID, req.PaymentMethod)

// 	if req.PaymentMethod != "COD" && req.PaymentMethod != "ONLINE" {
// 		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid payment method", "Choose either COD or ONLINE", "")
// 		return
// 	}

// 	var cart userModels.Cart
// 	var validCartItems []userModels.CartItem
// 	var address userModels.Address
// 	var finalPrice float64
// 	var couponDiscount float64
// 	var offerDiscount float64
// 	var coupon adminModels.Coupons
// 	orderID := helper.GenerateOrderID()
// 	totalPrice := 0.0
// 	err := database.DB.Transaction(func(tx *gorm.DB) error {

// 		var originalTotalPrice float64
// 		if err := tx.Where("user_id = ?", uid).
// 			Preload("CartItems.Product").
// 			Preload("CartItems.Variants").
// 			First(&cart).Error; err != nil {
// 			return fmt.Errorf("cart not found: %v", err)
// 		}

// 		for _, item := range cart.CartItems {
// 			var category adminModels.Category
// 			var variant adminModels.Variants

// 			isInStock := item.Product.IsListed && item.Product.InStock
// 			hasVariantStock := false
// 			if err := tx.Where("id = ? AND deleted_at IS NULL", item.VariantsID).First(&variant).Error; err == nil {
// 				hasVariantStock = variant.Stock >= item.Quantity
// 			}

// 			if isInStock && hasVariantStock &&
// 				tx.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error == nil {
// 				offerDetails := helper.GetBestOfferForProduct(&item.Product, item.Variants.ExtraPrice)
// 				item.Price = offerDetails.OriginalPrice
// 				item.DiscountedPrice = offerDetails.DiscountedPrice
// 				item.OriginalPrice = offerDetails.OriginalPrice
// 				item.DiscountPercentage = offerDetails.DiscountPercentage
// 				item.OfferName = offerDetails.OfferName
// 				item.IsOfferApplied = offerDetails.IsOfferApplied
// 				item.ItemTotal = offerDetails.DiscountedPrice * float64(item.Quantity)
// 				totalPrice += item.ItemTotal
// 				originalTotalPrice += offerDetails.OriginalPrice * float64(item.Quantity)

// 				itemOfferDiscount := (offerDetails.OriginalPrice - offerDetails.DiscountedPrice) * float64(item.Quantity)
// 				offerDiscount += itemOfferDiscount
// 				validCartItems = append(validCartItems, item)
// 			} else {
// 				log.Printf("Skipping item: product_id=%d, variant_id=%d, product_listed=%v, product_in_stock=%v, variant_stock=%d, quantity=%d",
// 					item.ProductID, item.VariantsID, item.Product.IsListed, isInStock, variant.Stock, item.Quantity)
// 			}
// 		}

// 		if len(validCartItems) == 0 {
// 			return fmt.Errorf("no valid items in cart with sufficient stock")
// 		}

// 		cart.OriginalTotalPrice = originalTotalPrice
// 		cart.TotalPrice = totalPrice
// 		if err := tx.Save(&cart).Error; err != nil {
// 			return fmt.Errorf("failed to save cart: %v", err)
// 		}

// 		if err := tx.Where("id = ? AND user_id = ?", req.AddressID, uid).First(&address).Error; err != nil {
// 			return fmt.Errorf("invalid address: %v", err)
// 		}

// 		finalPrice = cart.TotalPrice + shipping
// 		couponCode := ""
// 		if cart.CouponID != 0 {
// 			if err := tx.First(&coupon, cart.CouponID).Error; err == nil {
// 				if coupon.IsActive &&
// 					coupon.ExpiryDate.After(time.Now()) &&
// 					coupon.UsedCount < coupon.UsageLimit &&
// 					cart.OriginalTotalPrice >= coupon.MinPurchaseAmount {
// 					couponDiscount = cart.OriginalTotalPrice * (coupon.DiscountPercentage / 100)
// 					finalPrice -= couponDiscount
// 					couponCode = coupon.CouponCode
// 				} else {
// 					log.Printf("Coupon %s invalid: active=%v, expired=%v, used=%d/%d, min=%.2f, cartTotal=%.2f",
// 						coupon.CouponCode, coupon.IsActive, coupon.ExpiryDate.Before(time.Now()),
// 						coupon.UsedCount, coupon.UsageLimit, coupon.MinPurchaseAmount, cart.TotalPrice)
// 					cart.CouponID = 0
// 					if err := tx.Save(&cart).Error; err != nil {
// 						return fmt.Errorf("failed to reset coupon: %v", err)
// 					}
// 				}
// 			} else {
// 				log.Printf("Coupon ID %v not found: %v", cart.CouponID, err)
// 				cart.CouponID = 0
// 				if err := tx.Save(&cart).Error; err != nil {
// 					return fmt.Errorf("failed to reset coupon: %v", err)
// 				}
// 			}
// 		}

// 		if finalPrice < minimumOrderAmount {
// 			return fmt.Errorf("order amount too low: %.2f < %.2f", finalPrice, minimumOrderAmount)
// 		}

// 		var paymentStatus string
// 		if req.PaymentMethod == "COD" {
// 			paymentStatus = "Pending"
// 		} else {
// 			paymentStatus = "Created"
// 		}
// 		paymentDetails := adminModels.PaymentDetails{
// 			UserID:        uid,
// 			Amount:        finalPrice,
// 			AddressID:     req.AddressID,
// 			PaymentMethod: req.PaymentMethod,
// 			Status:        paymentStatus,
// 		}

// 		if req.PaymentMethod == "ONLINE" {
			
// 			razorpayOrder, err := client.Order.Create(data, nil)
// 			if err != nil {
// 				return fmt.Errorf("failed to create Razorpay order: %v", err)
// 			}
// 			paymentDetails.RazorpayOrderID = razorpayOrder["id"].(string)
// 			if err := tx.Create(&paymentDetails).Error; err != nil {
// 				return fmt.Errorf("failed to store payment details: %v", err)
// 			}
// 			c.Set("razorpayOrderID", paymentDetails.RazorpayOrderID)
// 			c.Set("amount", amount)
// 			return nil
// 		}

// 		// var orderCount int64
// 		// if err := tx.Model(&userModels.Orders{}).Where("user_id = ? AND payment_method = ?", uid, "COD").Count(&orderCount).Error; err != nil {
// 		// 	return fmt.Errorf("failed to check COD order count: %v", err)
// 		// }
// 		if err := tx.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address).Error; err != nil {
// 			return fmt.Errorf("invalid address: %v", err)
// 		}

// 		shippingAdd := adminModels.ShippingAddress{
// 			OrderID:        orderID,
// 			UserID:         uid,
// 			Name:           address.Name,
// 			City:           address.City,
// 			Landmark:       address.Landmark,
// 			State:          address.State,
// 			Pincode:        address.Pincode,
// 			AddressType:    address.AddressType,
// 			Phone:          address.Phone,
// 			AlternatePhone: address.AlternatePhone,
// 			TrackingStatus: "Pending",
// 		}
// 		if err := tx.Create(&shippingAdd).Error; err != nil {
// 			return fmt.Errorf("failed to create shipping address: %v", err)
// 		}

// 		order := userModels.Orders{
// 			UserID:          uid,
// 			OrderIdUnique:   orderID,
// 			AddressID:       req.AddressID,
// 			ShippingAddress: shippingAdd,
// 			TotalPrice:      finalPrice,
// 			Subtotal:        cart.OriginalTotalPrice,
// 			CouponDiscount:  couponDiscount,
// 			OfferDiscount:   offerDiscount,
// 			CouponID:        cart.CouponID,
// 			CouponCode:      couponCode,
// 			Status:          "Pending",
// 			PaymentMethod:   req.PaymentMethod,
// 			PaymentStatus:   paymentStatus,
// 			OrderDate:       time.Now(),
// 			ShippingCost:    shipping,

// 			CancellationStatus: "None",
// 		}

// 		if err := tx.Create(&order).Error; err != nil {
// 			return fmt.Errorf("failed to create order: %v", err)
// 		}
// 		orderBackup := userModels.OrderBackUp{
// 			ShippingCost:  order.ShippingCost,
// 			Subtotal:      cart.OriginalTotalPrice,
// 			TotalPrice:    finalPrice,
// 			OfferDiscount: offerDiscount,
// 			OrderIdUnique: orderID,
// 		}
// 		if err := tx.Create(&orderBackup).Error; err != nil {
// 			return fmt.Errorf("failed to create orderbackup: %v", err)
// 		}

// 		for _, item := range validCartItems {
// 			orderItem := userModels.OrderItem{
// 				OrderID:        order.ID,
// 				ProductID:      item.ProductID,
// 				VariantsID:     item.VariantsID,
// 				Quantity:       item.Quantity,
// 				UnitPrice:      item.DiscountedPrice,
// 				ItemTotal:      item.DiscountedPrice * float64(item.Quantity),
// 				DiscountAmount: (item.SalePrice - item.DiscountedPrice) * float64(item.Quantity),
// 				OfferName:      item.OfferName,
// 				Status:         "Active",
// 				ReturnStatus:   "None",
// 			}
// 			if err := tx.Create(&orderItem).Error; err != nil {
// 				return fmt.Errorf("failed to create order item: %v", err)
// 			}

// 			var variant adminModels.Variants
// 			if err := tx.First(&variant, item.VariantsID).Error; err != nil {
// 				return fmt.Errorf("variant not found: %v", err)
// 			}
// 			variant.Stock -= item.Quantity
// 			if variant.Stock == 0 {
// 				var product adminModels.Product
// 				if err := tx.First(&product, item.ProductID).Error; err != nil {
// 					return fmt.Errorf("product not found: %v", err)
// 				}
// 				product.InStock = false
// 				if err := tx.Save(&product).Error; err != nil {
// 					return fmt.Errorf("failed to update product stock: %v", err)
// 				}
// 			}
// 			if err := tx.Save(&variant).Error; err != nil {
// 				return fmt.Errorf("failed to update variant stock: %v", err)
// 			}
// 		}

// 		if cart.CouponID != 0 {
// 			coupon.UsedCount++
// 			if err := tx.Save(&coupon).Error; err != nil {
// 				return fmt.Errorf("failed to update coupon usage: %v", err)
// 			}
// 		}

// 		paymentDetails.OrderID = order.ID
// 		if err := tx.Create(&paymentDetails).Error; err != nil {
// 			return fmt.Errorf("failed to store payment details: %v", err)
// 		}
// 		if err := tx.Where("cart_id = ?", cart.ID).Delete(&userModels.CartItem{}).Error; err != nil {
// 			return fmt.Errorf("failed to delete cart items: %v", err)
// 		}
// 		if err := tx.Delete(&cart).Error; err != nil {
// 			return fmt.Errorf("failed to delete cart: %v", err)
// 		}

// 		return nil
// 	})

// 	if err != nil {
// 		log.Printf("Transaction error: %v", err)
// 		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to place order", err.Error(), "")
// 		return
// 	}

// 	if req.PaymentMethod == "ONLINE" {
// 		razorpayOrderID, exists := c.Get("razorpayOrderID")
// 		if !exists {
// 			log.Printf("Razorpay order ID not found in context")
// 			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to retrieve payment details", "Razorpay order ID not found", "")
// 			return
// 		}
// 		amount, exists := c.Get("amount")
// 		if !exists {
// 			log.Printf("Amount not found in context")
// 			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to retrieve payment details", "Amount not found", "")
// 			return
// 		}
// 		c.JSON(http.StatusOK, gin.H{
// 			"status":            "payment_required",
// 			"message":           "Proceed to payment",
// 			"razorpay_order_id": razorpayOrderID,
// 			"amount":            amount,
// 			"order_id":          orderID,
// 		})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"status":   "ok",
// 		"message":  "Order placed successfully",
// 		"order_id": orderID,
// 		"redirect": fmt.Sprintf("/order/success?order_id=%s", orderID),
// 	})
// }

// type PaymentVerification1 struct {
// 	RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
// 	RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
// 	RazorpaySignature string `json:"razorpay_signature" binding:"required"`
// }

// func VerifyPayment1(c *gin.Context) {
// 	userID, exists := c.Get("id")
// 	if !exists {
// 		log.Printf("No user ID found in context")
// 		c.JSON(http.StatusUnauthorized, gin.H{
// 			"status":   "error",
// 			"message":  "User not authenticated",
// 			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape("User not authenticated")),
// 		})
// 		return
// 	}
// 	uid, ok := userID.(uint)
// 	if !ok {
// 		log.Printf("Invalid user ID type")
// 		c.JSON(http.StatusInternalServerError, gin.H{
// 			"status":   "error",
// 			"message":  "Internal server error",
// 			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape("Invalid user ID type")),
// 		})
// 		return
// 	}

// 	var req PaymentVerification
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		log.Printf("Error binding JSON: %v", err)
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"status":   "error",
// 			"message":  "Invalid payment data",
// 			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape("Invalid payment data")),
// 		})
// 		return
// 	}

// 	if !helper.VerifyPaymentSignature(req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature) {
// 		log.Printf("Invalid payment signature for Razorpay order ID: %s", req.RazorpayOrderID)
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"status":   "error",
// 			"message":  "Invalid payment signature",
// 			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape("Invalid payment signature")),
// 		})
// 		return
// 	}

// 	var orderIdUnique string
// 	var cart userModels.Cart
// 	var validCartItems []userModels.CartItem
// 	var address userModels.Address
// 	var finalPrice float64
// 	var couponDiscount float64
// 	var offerDiscount float64
// 	var coupon adminModels.Coupons

// 	err := database.DB.Transaction(func(tx *gorm.DB) error {
// 		var paymentDetails adminModels.PaymentDetails
// 		if err := tx.Where("razorpay_order_id = ? AND user_id = ?", req.RazorpayOrderID, uid).First(&paymentDetails).Error; err != nil {
// 			return fmt.Errorf("payment record not found: %v", err)
// 		}

// 		if err := tx.Where("user_id = ?", uid).
// 			Preload("CartItems.Product").
// 			Preload("CartItems.Variants").
// 			First(&cart).Error; err != nil {
// 			return fmt.Errorf("cart not found: %v", err)
// 		}

// 		totalPrice := 0.0
// 		originalTotalPrice := 0.0
// 		totalDiscount := 0.0
// 		for _, item := range cart.CartItems {
// 			var category adminModels.Category
// 			var variant adminModels.Variants

// 			isInStock := item.Product.IsListed && item.Product.InStock
// 			hasVariantStock := false
// 			if err := tx.Where("id = ? AND deleted_at IS NULL", item.VariantsID).First(&variant).Error; err == nil {
// 				hasVariantStock = variant.Stock >= item.Quantity
// 			}

// 			if isInStock && hasVariantStock &&
// 				tx.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error == nil {
// 				offerDetails := helper.GetBestOfferForProduct(&item.Product, item.Variants.ExtraPrice)
// 				item.Price = offerDetails.OriginalPrice
// 				item.DiscountedPrice = offerDetails.DiscountedPrice
// 				// totalDiscount += offerDetails.DiscountedPrice + couponDiscount
// 				item.OriginalPrice = offerDetails.OriginalPrice
// 				item.DiscountPercentage = offerDetails.DiscountPercentage
// 				item.OfferName = offerDetails.OfferName
// 				item.IsOfferApplied = offerDetails.IsOfferApplied
// 				item.ItemTotal = offerDetails.DiscountedPrice * float64(item.Quantity)
// 				totalPrice += item.ItemTotal
// 				originalTotalPrice += offerDetails.OriginalPrice * float64(item.Quantity)
// 				itemOfferDiscount := (offerDetails.OriginalPrice - offerDetails.DiscountedPrice) * float64(item.Quantity)
// 				offerDiscount += itemOfferDiscount
// 				validCartItems = append(validCartItems, item)
// 			} else {
// 				log.Printf("Skipping item: product_id=%d, variant_id=%d, product_listed=%v, product_in_stock=%v, variant_stock=%d, quantity=%d",
// 					item.ProductID, item.VariantsID, item.Product.IsListed, isInStock, variant.Stock, item.Quantity)
// 			}
// 		}

// 		if len(validCartItems) == 0 {
// 			return fmt.Errorf("no valid items in cart with sufficient stock")
// 		}

// 		cart.TotalPrice = totalPrice
// 		cart.OriginalTotalPrice = originalTotalPrice
// 		if err := tx.Save(&cart).Error; err != nil {
// 			return fmt.Errorf("failed to save cart: %v", err)
// 		}

// 		finalPrice = cart.TotalPrice + shipping
// 		couponCode := ""
// 		if cart.CouponID != 0 {
// 			if err := tx.First(&coupon, cart.CouponID).Error; err == nil {
// 				if coupon.IsActive &&
// 					coupon.ExpiryDate.After(time.Now()) &&
// 					coupon.UsedCount < coupon.UsageLimit &&
// 					cart.OriginalTotalPrice >= coupon.MinPurchaseAmount {
// 					couponDiscount = cart.OriginalTotalPrice * (coupon.DiscountPercentage / 100)
// 					finalPrice -= couponDiscount
// 					couponCode = coupon.CouponCode
// 					totalDiscount += couponDiscount
// 				} else {
// 					log.Printf("Coupon %s invalid: active=%v, expired=%v, used=%d/%d, min=%.2f, cartOriginalTotal=%.2f",
// 						coupon.CouponCode, coupon.IsActive, coupon.ExpiryDate.Before(time.Now()),
// 						coupon.UsedCount, coupon.UsageLimit, coupon.MinPurchaseAmount, cart.OriginalTotalPrice)
// 					cart.CouponID = 0
// 					if err := tx.Save(&cart).Error; err != nil {
// 						return fmt.Errorf("failed to reset coupon: %v", err)
// 					}
// 				}
// 			} else {
// 				log.Printf("Coupon ID %v not found: %v", cart.CouponID, err)
// 				cart.CouponID = 0
// 				if err := tx.Save(&cart).Error; err != nil {
// 					return fmt.Errorf("failed to reset coupon: %v", err)
// 				}
// 			}
// 		}

// 		if finalPrice < minimumOrderAmount {
// 			return fmt.Errorf("order amount too low: %.2f < %.2f", finalPrice, minimumOrderAmount)
// 		}

// 		if err := tx.Where("id = ? AND user_id = ?", paymentDetails.AddressID, uid).First(&address).Error; err != nil {
// 			return fmt.Errorf("invalid address: %v", err)
// 		}

// 		orderID := helper.GenerateOrderID()
// 		shippingAdd := adminModels.ShippingAddress{
// 			OrderID:        orderID,
// 			UserID:         uid,
// 			Name:           address.Name,
// 			City:           address.City,
// 			Landmark:       address.Landmark,
// 			State:          address.State,
// 			Pincode:        address.Pincode,
// 			AddressType:    address.AddressType,
// 			Phone:          address.Phone,
// 			AlternatePhone: address.AlternatePhone,
// 			TrackingStatus: "Pending",
// 		}
// 		if err := tx.Create(&shippingAdd).Error; err != nil {
// 			return fmt.Errorf("failed to create shipping address: %v", err)
// 		}

// 		order := userModels.Orders{
// 			UserID:             uid,
// 			OrderIdUnique:      orderID,
// 			AddressID:          paymentDetails.AddressID,
// 			ShippingAddress:    shippingAdd,
// 			TotalPrice:         finalPrice,
// 			Subtotal:           cart.OriginalTotalPrice,
// 			CouponDiscount:     couponDiscount,
// 			OfferDiscount:      offerDiscount,
// 			CouponID:           cart.CouponID,
// 			CouponCode:         couponCode,
// 			Status:             "Confirmed",
// 			PaymentMethod:      "ONLINE",
// 			PaymentStatus:      "Paid",
// 			OrderDate:          time.Now(),
// 			TotalDiscount:      totalDiscount,
// 			ShippingCost:       shipping,
// 			CancellationStatus: "None",
// 		}
// 		if err := tx.Create(&order).Error; err != nil {
// 			return fmt.Errorf("failed to create order: %v", err)
// 		}

// 		orderBackup := userModels.OrderBackUp{
// 			ShippingCost:  order.ShippingCost,
// 			Subtotal:      cart.OriginalTotalPrice,
// 			TotalPrice:    finalPrice,
// 			OfferDiscount: offerDiscount,
// 			OrderIdUnique: orderID,
// 		}
// 		if err := tx.Create(&orderBackup).Error; err != nil {
// 			return fmt.Errorf("failed to create orderbackup: %v", err)
// 		}

// 		for _, item := range validCartItems {
// 			orderItem := userModels.OrderItem{
// 				OrderID:        order.ID,
// 				ProductID:      item.ProductID,
// 				VariantsID:     item.VariantsID,
// 				Quantity:       item.Quantity,
// 				UnitPrice:      item.DiscountedPrice,
// 				ItemTotal:      item.DiscountedPrice * float64(item.Quantity),
// 				DiscountAmount: (item.OriginalPrice - item.DiscountedPrice) * float64(item.Quantity),
// 				OfferName:      item.OfferName,
// 				Status:         "Active",
// 				ReturnStatus:   "None",
// 			}
// 			if err := tx.Create(&orderItem).Error; err != nil {
// 				return fmt.Errorf("failed to create order item: %v", err)
// 			}

// 			var variant adminModels.Variants
// 			if err := tx.First(&variant, item.VariantsID).Error; err != nil {
// 				return fmt.Errorf("variant not found: %v", err)
// 			}
// 			if variant.Stock < item.Quantity {
// 				return fmt.Errorf("insufficient stock for variant ID %d", item.VariantsID)
// 			}
// 			variant.Stock -= item.Quantity
// 			if variant.Stock == 0 {
// 				var product adminModels.Product
// 				if err := tx.First(&product, item.ProductID).Error; err != nil {
// 					return fmt.Errorf("product not found: %v", err)
// 				}
// 				product.InStock = false
// 				if err := tx.Save(&product).Error; err != nil {
// 					return fmt.Errorf("failed to update product stock: %v", err)
// 				}
// 			}
// 			if err := tx.Save(&variant).Error; err != nil {
// 				return fmt.Errorf("failed to update variant stock: %v", err)
// 			}
// 		}

// 		if cart.CouponID != 0 {
// 			coupon.UsedCount++
// 			if err := tx.Save(&coupon).Error; err != nil {
// 				return fmt.Errorf("failed to update coupon usage: %v", err)
// 			}
// 		}

// 		paymentDetails.OrderID = order.ID
// 		paymentDetails.RazorpayPaymentID = req.RazorpayPaymentID
// 		paymentDetails.RazorpaySignature = req.RazorpaySignature
// 		paymentDetails.Status = "Paid"
// 		paymentDetails.Attempts++
// 		if err := tx.Save(&paymentDetails).Error; err != nil {
// 			return fmt.Errorf("failed to update payment status: %v", err)
// 		}

// 		if err := tx.Where("cart_id = ?", cart.ID).Delete(&userModels.CartItem{}).Error; err != nil {
// 			return fmt.Errorf("failed to delete cart items: %v", err)
// 		}
// 		if err := tx.Delete(&cart).Error; err != nil {
// 			return fmt.Errorf("failed to delete cart: %v", err)
// 		}

// 		orderIdUnique = order.OrderIdUnique
// 		return nil
// 	})

// 	if err != nil {
// 		log.Printf("Transaction error: %v", err)
// 		c.JSON(http.StatusInternalServerError, gin.H{
// 			"status":   "error",
// 			"message":  "Failed to verify payment",
// 			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape(err.Error())),
// 		})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"status":   "ok",
// 		"message":  "Payment verified successfully",
// 		"order_id": orderIdUnique,
// 		"redirect": fmt.Sprintf("/order/success?order_id=%s", orderIdUnique),
// 	})
// }
