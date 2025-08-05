package controllers

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"github.com/razorpay/razorpay-go"
	"gorm.io/gorm"
)

const shipping = 10.0
const minimumOrderAmount = 10.0

func ShowCheckout(c *gin.Context) {
	userID, _ := c.Get("id")

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).
		Preload("CartItems.Product").
		Preload("CartItems.Variants").
		First(&cart).Error; err != nil {
		log.Printf("Cart not found for user %v: %v", userID, err)
		if err := database.DB.Where("user_id = ? AND status = ?", userID, "Pending").
			Order("order_date DESC").
			First(&userModels.Orders{}).Error; err == nil {
			var recentOrder userModels.Orders
			database.DB.Where("user_id = ? AND status = ?", userID, "Pending").
				Order("order_date DESC").
				First(&recentOrder)
			c.Redirect(http.StatusFound, fmt.Sprintf("/order/success?order_id=%s", recentOrder.OrderIdUnique))
			return
		}
		c.Redirect(http.StatusFound, "/cart")
		return
	}

	var validCartItems []userModels.CartItem
	var invalidCartItems []userModels.CartItem
	totalPrice := 0.0
	offerDiscount := 0.0
	originalTotalPrice := 0.0

	for _, item := range cart.CartItems {
		var category adminModels.Category
		var variant adminModels.Variants

		isInStock := item.Product.IsListed && item.Product.InStock
		hasVariantStock := false
		if err := database.DB.Where("id = ? AND deleted_at IS NULL", item.VariantsID).First(&variant).Error; err == nil {
			hasVariantStock = variant.Stock >= item.Quantity
		}

		if isInStock && hasVariantStock &&
			database.DB.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error == nil {
			offerDetails := helper.GetBestOfferForProduct(&item.Product, item.Variants.ExtraPrice)
			item.Price = offerDetails.OriginalPrice
			item.DiscountedPrice = offerDetails.DiscountedPrice
			item.OriginalPrice = offerDetails.OriginalPrice
			item.DiscountPercentage = offerDetails.DiscountPercentage
			item.OfferName = offerDetails.OfferName
			item.IsOfferApplied = offerDetails.IsOfferApplied

			// Calculate offer discount for this item
			itemOfferDiscount := (offerDetails.OriginalPrice - offerDetails.DiscountedPrice) * float64(item.Quantity)
			offerDiscount += itemOfferDiscount

			item.ItemTotal = offerDetails.DiscountedPrice * float64(item.Quantity)
			totalPrice += item.ItemTotal
			originalTotalPrice += offerDetails.OriginalPrice * float64(item.Quantity)
			validCartItems = append(validCartItems, item)
		} else {
			item.Product.InStock = false
			item.Variants.Stock = variant.Stock
			invalidCartItems = append(invalidCartItems, item)
			log.Printf("Invalid cart item for user %v: product_id=%d, variant_id=%d, product_listed=%v, product_in_stock=%v, variant_stock=%d, quantity=%d, category_error=%v",
				userID, item.ProductID, item.VariantsID, item.Product.IsListed, isInStock, variant.Stock, item.Quantity,
				database.DB.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error)
		}
	}

	if len(validCartItems) == 0 {
		log.Printf("No valid items in cart for user %v", userID)
		helper.ResponseWithErr(c, http.StatusBadRequest, "No valid products in cart",
			"All products in your cart are either out of stock or have insufficient quantity", "")
		return
	}

	if totalPrice != cart.TotalPrice {
		cart.TotalPrice = totalPrice
		if err := database.DB.Save(&cart).Error; err != nil {
			log.Printf("Failed to update cart total for user %v: %v", userID, err)
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update cart", err.Error(), "")
			return
		}
	}

	cart.CartItems = append(validCartItems, invalidCartItems...)

	var addresses []userModels.Address
	if err := database.DB.Where("user_id = ?", userID).Find(&addresses).Error; err != nil {
		log.Printf("Failed to load addresses for user %v: %v", userID, err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to load addresses", err.Error(), "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		log.Printf("Failed to load user details for user %v: %v", userID, err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to load user details", err.Error(), "")
		return
	}

	finalPrice := totalPrice + shipping
	var appliedCoupon adminModels.Coupons
	var couponDiscount float64 = 0.0
	var couponApplied bool

	if cart.CouponID != 0 {
		if err := database.DB.First(&appliedCoupon, cart.CouponID).Error; err == nil {
			if appliedCoupon.IsActive &&
				appliedCoupon.ExpiryDate.After(time.Now()) &&
				appliedCoupon.UsedCount < appliedCoupon.UsageLimit &&
				totalPrice >= appliedCoupon.MinPurchaseAmount {
				// Calculate coupon discount based on the discounted total (after offers)
				couponDiscount = originalTotalPrice * (appliedCoupon.DiscountPercentage / 100)
				finalPrice -= couponDiscount
				couponApplied = true
				log.Printf("Coupon %s applied: discount=%.2f, finalPrice=%.2f", appliedCoupon.CouponCode, couponDiscount, finalPrice)
			} else {
				log.Printf("Coupon %s invalid: active=%v, expired=%v, used=%d/%d, min=%.2f, cartTotal=%.2f",
					appliedCoupon.CouponCode, appliedCoupon.IsActive, appliedCoupon.ExpiryDate.Before(time.Now()),
					appliedCoupon.UsedCount, appliedCoupon.UsageLimit, appliedCoupon.MinPurchaseAmount, totalPrice)
				cart.CouponID = 0
				if err := database.DB.Save(&cart).Error; err != nil {
					log.Printf("Failed to reset coupon for cart %v: %v", cart.ID, err)
					helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update cart", err.Error(), "")
					return
				}
			}
		} else {
			log.Printf("Coupon ID %v not found for cart %v: %v", cart.CouponID, cart.ID, err)
			cart.CouponID = 0
			if err := database.DB.Save(&cart).Error; err != nil {
				log.Printf("Failed to reset coupon for cart %v: %v", cart.ID, err)
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update cart", err.Error(), "")
				return
			}
		}
	}

	if finalPrice < minimumOrderAmount {
		log.Printf("Order amount too low for user %v: finalPrice=%.2f, minimum=%.2f", userID, finalPrice, minimumOrderAmount)
		helper.ResponseWithErr(c, http.StatusBadRequest, "Order amount too low",
			fmt.Sprintf("Order amount must be at least ₹%.2f", minimumOrderAmount), "")
		return
	}

	userr, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "checkout.html", gin.H{
			"title":              "Checkout",
			"Cart":               cart,
			"InvalidCartItems":   invalidCartItems,
			"HasInvalidItems":    len(invalidCartItems) > 0,
			"Addresses":          addresses,
			"Shipping":           shipping,
			"FinalPrice":         finalPrice,
			"Subtotal":           originalTotalPrice, // This should be the discounted subtotal
			"OriginalTotalPrice": originalTotalPrice,
			"TotalDiscount":      offerDiscount + couponDiscount, // FIXED: Use totalDiscount instead of Discount
			"OfferDiscount":      offerDiscount,                  // Separate offer discount
			"CouponDiscount":     couponDiscount,                 // Separate coupon discount
			"CouponApplied":      couponApplied,
			"AppliedCoupon":      appliedCoupon,
			"UserEmail":          user.Email,
			"UserPhone":          user.Phone,
			"RazorpayKey":        os.Getenv("RAZORPAY_KEY_ID"),
			"status":             "success",
			"UserName":           "Guest",
			"WishlistCount":      0,
			"CartCount":          0,
			"ProfileImage":       "",
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

	c.HTML(http.StatusOK, "checkout.html", gin.H{
		"title":              "Checkout",
		"Cart":               cart,
		"InvalidCartItems":   invalidCartItems,
		"HasInvalidItems":    len(invalidCartItems) > 0,
		"Addresses":          addresses,
		"Shipping":           shipping,
		"FinalPrice":         finalPrice,
		"Subtotal":           originalTotalPrice,
		"OriginalTotalPrice": originalTotalPrice,
		"TotalDiscount":      offerDiscount + couponDiscount, // FIXED: Use totalDiscount instead of Discount
		"OfferDiscount":      offerDiscount,                  // Separate offer discount
		"CouponDiscount":     couponDiscount,                 // Separate coupon discount
		"CouponApplied":      couponApplied,
		"AppliedCoupon":      appliedCoupon,
		"UserEmail":          user.Email,
		"UserPhone":          user.Phone,
		"RazorpayKey":        os.Getenv("RAZORPAY_KEY_ID"),
		"UserName":           userNameStr,
		"ProfileImage":       userData.ProfileImage,
		"WishlistCount":      wishlistCount,
		"CartCount":          cartCount,
	})
}

type PaymentRequest struct {
	AddressID     uint   `json:"address_id" binding:"required"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}

func PlaceOrder(c *gin.Context) {
	userID, exists := c.Get("id")
	if !exists {
		log.Printf("No user ID found in context")
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User not authenticated", "No user ID found in context", "")
		return
	}
	uid, ok := userID.(uint)
	if !ok {
		log.Printf("Invalid user ID type")
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Internal server error", "Invalid user ID type", "")
		return
	}

	var req PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON: %v", err)
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	log.Printf("PlaceOrder request for user %v: address_id=%d, payment_method=%s", uid, req.AddressID, req.PaymentMethod)

	if req.PaymentMethod != "COD" && req.PaymentMethod != "ONLINE" {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid payment method", "Choose either COD or ONLINE", "")
		return
	}

	var cart userModels.Cart
	var validCartItems []userModels.CartItem
	var address userModels.Address
	var finalPrice float64
	var couponDiscount float64
	var offerDiscount float64
	var coupon adminModels.Coupons
	orderID := helper.GenerateOrderID()
	totalPrice := 0.0
	err := database.DB.Transaction(func(tx *gorm.DB) error {

		var originalTotalPrice float64
		if err := tx.Where("user_id = ?", uid).
			Preload("CartItems.Product").
			Preload("CartItems.Variants").
			First(&cart).Error; err != nil {
			return fmt.Errorf("cart not found: %v", err)
		}

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
				originalTotalPrice += offerDetails.OriginalPrice * float64(item.Quantity)

				itemOfferDiscount := (offerDetails.OriginalPrice - offerDetails.DiscountedPrice) * float64(item.Quantity)
				offerDiscount += itemOfferDiscount
				validCartItems = append(validCartItems, item)
			} else {
				log.Printf("Skipping item: product_id=%d, variant_id=%d, product_listed=%v, product_in_stock=%v, variant_stock=%d, quantity=%d",
					item.ProductID, item.VariantsID, item.Product.IsListed, isInStock, variant.Stock, item.Quantity)
			}
		}

		if len(validCartItems) == 0 {
			return fmt.Errorf("no valid items in cart with sufficient stock")
		}

		cart.OriginalTotalPrice = originalTotalPrice
		cart.TotalPrice = totalPrice
		if err := tx.Save(&cart).Error; err != nil {
			return fmt.Errorf("failed to save cart: %v", err)
		}

		if err := tx.Where("id = ? AND user_id = ?", req.AddressID, uid).First(&address).Error; err != nil {
			return fmt.Errorf("invalid address: %v", err)
		}

		finalPrice = cart.TotalPrice + shipping
		couponCode := ""
		if cart.CouponID != 0 {
			if err := tx.First(&coupon, cart.CouponID).Error; err == nil {
				if coupon.IsActive &&
					coupon.ExpiryDate.After(time.Now()) &&
					coupon.UsedCount < coupon.UsageLimit &&
					cart.OriginalTotalPrice >= coupon.MinPurchaseAmount {
					couponDiscount = cart.OriginalTotalPrice * (coupon.DiscountPercentage / 100)
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

		if finalPrice < minimumOrderAmount {
			return fmt.Errorf("order amount too low: %.2f < %.2f", finalPrice, minimumOrderAmount)
		}

		var paymentStatus string
		if req.PaymentMethod == "COD" {
			paymentStatus = "Pending"
		} else {
			paymentStatus = "Created"
		}
		paymentDetails := adminModels.PaymentDetails{
			UserID:        uid,
			Amount:        finalPrice,
			AddressID:     req.AddressID,
			PaymentMethod: req.PaymentMethod,
			Status:        paymentStatus,
		}

		if req.PaymentMethod == "ONLINE" {
			client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))
			amount := int(finalPrice * 100)
			data := map[string]interface{}{
				"amount":   amount,
				"currency": "INR",
				"receipt":  fmt.Sprintf("receipt_%d_%d", uid, time.Now().Unix()),
			}
			razorpayOrder, err := client.Order.Create(data, nil)
			if err != nil {
				return fmt.Errorf("failed to create Razorpay order: %v", err)
			}
			paymentDetails.RazorpayOrderID = razorpayOrder["id"].(string)
			if err := tx.Create(&paymentDetails).Error; err != nil {
				return fmt.Errorf("failed to store payment details: %v", err)
			}
			c.Set("razorpayOrderID", paymentDetails.RazorpayOrderID)
			c.Set("amount", amount)
			return nil
		}

		var orderCount int64
		if err := tx.Model(&userModels.Orders{}).Where("user_id = ? AND payment_method = ?", uid, "COD").Count(&orderCount).Error; err != nil {
			return fmt.Errorf("failed to check COD order count: %v", err)
		}
		if err := tx.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address).Error; err != nil {
			return fmt.Errorf("invalid address: %v", err)
		}

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

		order := userModels.Orders{
			UserID:          uid,
			OrderIdUnique:   orderID,
			AddressID:       req.AddressID,
			ShippingAddress: shippingAdd,
			TotalPrice:      finalPrice,
			Subtotal:        cart.OriginalTotalPrice,
			CouponDiscount:  couponDiscount,
			OfferDiscount:   offerDiscount,
			CouponID:        cart.CouponID,
			CouponCode:      couponCode,
			Status:          "Pending",
			PaymentMethod:   req.PaymentMethod,
			PaymentStatus:   paymentStatus,
			OrderDate:       time.Now(),
			ShippingCost:    shipping,

			CancellationStatus: "None",
		}

		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("failed to create order: %v", err)
		}
		orderBackup := userModels.OrderBackUp{
			ShippingCost:  order.ShippingCost,
			Subtotal:      cart.OriginalTotalPrice,
			TotalPrice:    finalPrice,
			OfferDiscount: offerDiscount,
			OrderIdUnique: orderID,
		}
		if err := tx.Create(&orderBackup).Error; err != nil {
			return fmt.Errorf("failed to create orderbackup: %v", err)
		}

		for _, item := range validCartItems {
			orderItem := userModels.OrderItem{
				OrderID:        order.ID,
				ProductID:      item.ProductID,
				VariantsID:     item.VariantsID,
				Quantity:       item.Quantity,
				UnitPrice:      item.DiscountedPrice,
				ItemTotal:      item.DiscountedPrice * float64(item.Quantity),
				DiscountAmount: (item.OriginalPrice - item.DiscountedPrice) * float64(item.Quantity),
				OfferName:      item.OfferName,
				Status:         "Active",
				ReturnStatus:   "None",
			}
			if err := tx.Create(&orderItem).Error; err != nil {
				return fmt.Errorf("failed to create order item: %v", err)
			}

			var variant adminModels.Variants
			if err := tx.First(&variant, item.VariantsID).Error; err != nil {
				return fmt.Errorf("variant not found: %v", err)
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

		if cart.CouponID != 0 {
			coupon.UsedCount++
			if err := tx.Save(&coupon).Error; err != nil {
				return fmt.Errorf("failed to update coupon usage: %v", err)
			}
		}

		paymentDetails.OrderID = order.ID
		if err := tx.Create(&paymentDetails).Error; err != nil {
			return fmt.Errorf("failed to store payment details: %v", err)
		}
		if err := tx.Where("cart_id = ?", cart.ID).Delete(&userModels.CartItem{}).Error; err != nil {
			return fmt.Errorf("failed to delete cart items: %v", err)
		}
		if err := tx.Delete(&cart).Error; err != nil {
			return fmt.Errorf("failed to delete cart: %v", err)
		}

		return nil
	})

	if err != nil {
		log.Printf("Transaction error: %v", err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to place order", err.Error(), "")
		return
	}

	if req.PaymentMethod == "ONLINE" {
		razorpayOrderID, exists := c.Get("razorpayOrderID")
		if !exists {
			log.Printf("Razorpay order ID not found in context")
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to retrieve payment details", "Razorpay order ID not found", "")
			return
		}
		amount, exists := c.Get("amount")
		if !exists {
			log.Printf("Amount not found in context")
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to retrieve payment details", "Amount not found", "")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":            "payment_required",
			"message":           "Proceed to payment",
			"razorpay_order_id": razorpayOrderID,
			"amount":            amount,
			"order_id":          orderID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "Order placed successfully",
		"order_id": orderID,
		"redirect": fmt.Sprintf("/order/success?order_id=%s", orderID),
	})
}

type PaymentVerification struct {
	RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
	RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
	RazorpaySignature string `json:"razorpay_signature" binding:"required"`
}

func VerifyPayment(c *gin.Context) {
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

	if !helper.VerifyPaymentSignature(req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature) {
		log.Printf("Invalid payment signature for Razorpay order ID: %s", req.RazorpayOrderID)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":   "error",
			"message":  "Invalid payment signature",
			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape("Invalid payment signature")),
		})
		return
	}

	var orderIdUnique string
	var cart userModels.Cart
	var validCartItems []userModels.CartItem
	var address userModels.Address
	var finalPrice float64
	var couponDiscount float64
	var offerDiscount float64
	var coupon adminModels.Coupons

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var paymentDetails adminModels.PaymentDetails
		if err := tx.Where("razorpay_order_id = ? AND user_id = ?", req.RazorpayOrderID, uid).First(&paymentDetails).Error; err != nil {
			return fmt.Errorf("payment record not found: %v", err)
		}

		if err := tx.Where("user_id = ?", uid).
			Preload("CartItems.Product").
			Preload("CartItems.Variants").
			First(&cart).Error; err != nil {
			return fmt.Errorf("cart not found: %v", err)
		}

		totalPrice := 0.0
		originalTotalPrice := 0.0
		totalDiscount := 0.0
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
				// totalDiscount += offerDetails.DiscountedPrice + couponDiscount
				item.OriginalPrice = offerDetails.OriginalPrice
				item.DiscountPercentage = offerDetails.DiscountPercentage
				item.OfferName = offerDetails.OfferName
				item.IsOfferApplied = offerDetails.IsOfferApplied
				item.ItemTotal = offerDetails.DiscountedPrice * float64(item.Quantity)
				totalPrice += item.ItemTotal
				originalTotalPrice += offerDetails.OriginalPrice * float64(item.Quantity)
				itemOfferDiscount := (offerDetails.OriginalPrice - offerDetails.DiscountedPrice) * float64(item.Quantity)
				offerDiscount += itemOfferDiscount
				validCartItems = append(validCartItems, item)
			} else {
				log.Printf("Skipping item: product_id=%d, variant_id=%d, product_listed=%v, product_in_stock=%v, variant_stock=%d, quantity=%d",
					item.ProductID, item.VariantsID, item.Product.IsListed, isInStock, variant.Stock, item.Quantity)
			}
		}

		if len(validCartItems) == 0 {
			return fmt.Errorf("no valid items in cart with sufficient stock")
		}

		cart.TotalPrice = totalPrice
		cart.OriginalTotalPrice = originalTotalPrice
		if err := tx.Save(&cart).Error; err != nil {
			return fmt.Errorf("failed to save cart: %v", err)
		}

		finalPrice = cart.TotalPrice + shipping
		couponCode := ""
		if cart.CouponID != 0 {
			if err := tx.First(&coupon, cart.CouponID).Error; err == nil {
				if coupon.IsActive &&
					coupon.ExpiryDate.After(time.Now()) &&
					coupon.UsedCount < coupon.UsageLimit &&
					cart.OriginalTotalPrice >= coupon.MinPurchaseAmount {
					couponDiscount = cart.OriginalTotalPrice * (coupon.DiscountPercentage / 100)
					finalPrice -= couponDiscount
					couponCode = coupon.CouponCode
					totalDiscount += couponDiscount
				} else {
					log.Printf("Coupon %s invalid: active=%v, expired=%v, used=%d/%d, min=%.2f, cartOriginalTotal=%.2f",
						coupon.CouponCode, coupon.IsActive, coupon.ExpiryDate.Before(time.Now()),
						coupon.UsedCount, coupon.UsageLimit, coupon.MinPurchaseAmount, cart.OriginalTotalPrice)
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

		if finalPrice < minimumOrderAmount {
			return fmt.Errorf("order amount too low: %.2f < %.2f", finalPrice, minimumOrderAmount)
		}

		if err := tx.Where("id = ? AND user_id = ?", paymentDetails.AddressID, uid).First(&address).Error; err != nil {
			return fmt.Errorf("invalid address: %v", err)
		}

		orderID := helper.GenerateOrderID()
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

		order := userModels.Orders{
			UserID:             uid,
			OrderIdUnique:      orderID,
			AddressID:          paymentDetails.AddressID,
			ShippingAddress:    shippingAdd,
			TotalPrice:         finalPrice,
			Subtotal:           cart.OriginalTotalPrice,
			CouponDiscount:     couponDiscount,
			OfferDiscount:      offerDiscount,
			CouponID:           cart.CouponID,
			CouponCode:         couponCode,
			Status:             "Confirmed",
			PaymentMethod:      "ONLINE",
			PaymentStatus:      "Paid",
			OrderDate:          time.Now(),
			TotalDiscount:      totalDiscount,
			ShippingCost:       shipping,
			CancellationStatus: "None",
		}
		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("failed to create order: %v", err)
		}

		orderBackup := userModels.OrderBackUp{
			ShippingCost:  order.ShippingCost,
			Subtotal:      cart.OriginalTotalPrice,
			TotalPrice:    finalPrice,
			OfferDiscount: offerDiscount,
			OrderIdUnique: orderID,
		}
		if err := tx.Create(&orderBackup).Error; err != nil {
			return fmt.Errorf("failed to create orderbackup: %v", err)
		}

		for _, item := range validCartItems {
			orderItem := userModels.OrderItem{
				OrderID:        order.ID,
				ProductID:      item.ProductID,
				VariantsID:     item.VariantsID,
				Quantity:       item.Quantity,
				UnitPrice:      item.DiscountedPrice,
				ItemTotal:      item.DiscountedPrice * float64(item.Quantity),
				DiscountAmount: (item.OriginalPrice - item.DiscountedPrice) * float64(item.Quantity),
				OfferName:      item.OfferName,
				Status:         "Active",
				ReturnStatus:   "None",
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

		if cart.CouponID != 0 {
			coupon.UsedCount++
			if err := tx.Save(&coupon).Error; err != nil {
				return fmt.Errorf("failed to update coupon usage: %v", err)
			}
		}

		paymentDetails.OrderID = order.ID
		paymentDetails.RazorpayPaymentID = req.RazorpayPaymentID
		paymentDetails.RazorpaySignature = req.RazorpaySignature
		paymentDetails.Status = "Paid"
		paymentDetails.Attempts++
		if err := tx.Save(&paymentDetails).Error; err != nil {
			return fmt.Errorf("failed to update payment status: %v", err)
		}

		if err := tx.Where("cart_id = ?", cart.ID).Delete(&userModels.CartItem{}).Error; err != nil {
			return fmt.Errorf("failed to delete cart items: %v", err)
		}
		if err := tx.Delete(&cart).Error; err != nil {
			return fmt.Errorf("failed to delete cart: %v", err)
		}

		orderIdUnique = order.OrderIdUnique
		return nil
	})

	if err != nil {
		log.Printf("Transaction error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":   "error",
			"message":  "Failed to verify payment",
			"redirect": fmt.Sprintf("/order/failure?error=%s", url.QueryEscape(err.Error())),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "Payment verified successfully",
		"order_id": orderIdUnique,
		"redirect": fmt.Sprintf("/order/success?order_id=%s", orderIdUnique),
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
		log.Printf("Failed to set default address for user %v: %v", userID, err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to set default address", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Default address updated"})
}

func ShowOrderSuccess(c *gin.Context) {
	userID, _ := c.Get("id")
	orderID := c.Query("order_id")
	var order userModels.Orders
	if err := database.DB.Where("order_id_unique = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
		log.Printf("Failed to load order %s for user %v: %v", orderID, userID, err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to load order", err.Error(), "")
		return
	}
	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "home.html", gin.H{
			"status":        "success",
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

	c.HTML(http.StatusOK, "orderSuccess.html", gin.H{
		"title":         "Order Successful",
		"OrderID":       order.OrderIdUnique,
		"status":        "success",
		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
	})
}

func ShowOrderFailure(c *gin.Context) {
	userID, _ := c.Get("id")
	errorMsg := c.Query("error")
	orderID := c.Query("order_id")

	var order userModels.Orders
	var orderExists bool

	if orderID != "" {
		if err := database.DB.Where("order_id_unique = ? AND user_id = ?", orderID, userID).
			First(&order).Error; err == nil {
			orderExists = true
		}
	}

	if errorMsg == "" {
		errorMsg = "An error occurred while processing your payment."
	}

	c.HTML(http.StatusOK, "orderFailed.html", gin.H{
		"title":       "Order Failed",
		"Error":       errorMsg,
		"OrderID":     orderID,
		"OrderExists": orderExists,
	})
}
