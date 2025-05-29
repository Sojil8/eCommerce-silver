package controllers

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
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
		// helper.ResponseWithErr(c, http.StatusNotFound, "Cart not found", "Please add items to cart", "")
		if err := database.DB.Where("user_id = ? AND status = ?", userID, "Pending").
            Order("order_date DESC").
            First(&userModels.Orders{}).Error; err == nil {
            // Fetch the order ID to redirect
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
	var discount float64
	var couponApplied bool

	if cart.CouponID != 0 {
		if err := database.DB.First(&appliedCoupon, cart.CouponID).Error; err == nil {
			if appliedCoupon.IsActive &&
				appliedCoupon.ExpiryDate.After(time.Now()) &&
				appliedCoupon.UsedCount < appliedCoupon.UsageLimit &&
				totalPrice >= appliedCoupon.MinPurchaseAmount {
				discount = totalPrice * (appliedCoupon.DiscountPercentage / 100)
				finalPrice -= discount
				couponApplied = true
				log.Printf("Coupon %s applied: discount=%.2f, finalPrice=%.2f", appliedCoupon.CouponCode, discount, finalPrice)
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
			"Subtotal":           totalPrice,
			"OriginalTotalPrice": originalTotalPrice,
			"Discount":           discount,
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
		"Subtotal":           totalPrice,
		"OriginalTotalPrice": originalTotalPrice,
		"Discount":           discount,
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
	// 1. Extract and validate user ID
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

	// 2. Bind JSON request
	var req PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON: %v", err)
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	log.Printf("PlaceOrder request for user %v: address_id=%d, payment_method=%s", uid, req.AddressID, req.PaymentMethod)

	// 3. Validate payment method
	if req.PaymentMethod != "COD" && req.PaymentMethod != "ONLINE" {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid payment method", "Choose either COD or ONLINE", "")
		return
	}

	// 4. Initialize variables
	var cart userModels.Cart
	var validCartItems []userModels.CartItem
	var address userModels.Address
	var finalPrice float64
	var couponDiscount float64
	var offerDiscount float64
	var coupon adminModels.Coupons
	orderID := generateOrderID()
	totalPrice := 0.0

	// 5. Single transaction for validation and order creation (for COD only)
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Validate cart
		if err := tx.Where("user_id = ?", uid).
			Preload("CartItems.Product").
			Preload("CartItems.Variants").
			First(&cart).Error; err != nil {
			return fmt.Errorf("cart not found: %v", err)
		}

		// Validate cart items and calculate offer discounts
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

		// Validate address
		if err := tx.Where("id = ? AND user_id = ?", req.AddressID, uid).First(&address).Error; err != nil {
			return fmt.Errorf("invalid address: %v", err)
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

		// Create payment details (for both COD and ONLINE)
		var paymentStatus string
		if req.PaymentMethod == "COD" {
			paymentStatus = "Pending"
		} else {
			paymentStatus = "Created"
		}
		paymentDetails := adminModels.PaymentDetails{
			UserID:        uid,
			Amount:        finalPrice,
			AddressID: req.AddressID,
			PaymentMethod: req.PaymentMethod,
			Status:        paymentStatus,
		}

		if req.PaymentMethod == "ONLINE" {
			// For ONLINE, only create payment details and return
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

		// For COD, proceed with order creation
		// Validate COD order limit
		var orderCount int64
		if err := tx.Model(&userModels.Orders{}).Where("user_id = ? AND payment_method = ?", uid, "COD").Count(&orderCount).Error; err != nil {
			return fmt.Errorf("failed to check COD order count: %v", err)
		}
		if err := tx.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address).Error; err != nil {
			return fmt.Errorf("invalid address: %v", err)
		}

		// Create shipping address
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
			AddressID:             req.AddressID,
			ShippingAddress:       shippingAdd,
			TotalPrice:            finalPrice,
			Subtotal:              cart.TotalPrice,
			CouponDiscount:        couponDiscount,
			OfferDiscount:         offerDiscount,
			CouponID:              cart.CouponID,
			CouponCode:            couponCode,
			Status:                "Pending",
			PaymentMethod:         req.PaymentMethod,
			PaymentStatus:         paymentStatus,
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
				// VariantAttributes: variantAttributes,
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

		// Update coupon usage for COD
		if cart.CouponID != 0 {
			coupon.UsedCount++
			if err := tx.Save(&coupon).Error; err != nil {
				return fmt.Errorf("failed to update coupon usage: %v", err)
			}
		}

		// Store payment details for COD
		paymentDetails.OrderID = order.ID
		if err := tx.Create(&paymentDetails).Error; err != nil {
			return fmt.Errorf("failed to store payment details: %v", err)
		}

		// Delete cart for COD
		if err := tx.Delete(&cart).Error; err != nil {
			return fmt.Errorf("failed to delete cart: %v", err)
		}

		return nil
	})

	// 6. Handle transaction error
	if err != nil {
		log.Printf("Transaction error: %v", err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to place order", err.Error(), "")
		return
	}

	// 7. Prepare response
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

func generateOrderID() string {
	timestamp := time.Now().Format("20060102")
	randomNum := rand.Intn(10000)
	return fmt.Sprintf("ORD-%s-%04d", timestamp, randomNum)
}
