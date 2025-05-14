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
const minimumOrderAmount = 1.0

func ShowCheckout(c *gin.Context) {
	userID, _ := c.Get("id")

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).
		Preload("CartItems.Product").
		Preload("CartItems.Variants").
		First(&cart).Error; err != nil {
		log.Printf("Cart not found for user %v: %v", userID, err)
		helper.ResponseWithErr(c, http.StatusNotFound, "Cart not found", "Please add items to cart", "")
		return
	}

	var validCartItems []userModels.CartItem
	var invalidProductFound bool
	totalPrice := 0.0
	originalTotalPrice := 0.0 // New variable for original total

	for _, item := range cart.CartItems {
		var category adminModels.Category
		var variant adminModels.Variants
		if item.Product.IsListed &&
			database.DB.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error == nil &&
			database.DB.Where("id = ? AND deleted_at IS NULL", item.VariantsID).First(&variant).Error == nil {
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
			invalidProductFound = true
			log.Printf("Invalid cart item for user %v: product_id=%d, variant_id=%d, product_listed=%v, category_error=%v, variant_error=%v",
				userID, item.ProductID, item.VariantsID, item.Product.IsListed,
				database.DB.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error,
				database.DB.Where("id = ? AND deleted_at IS NULL", item.VariantsID).First(&variant).Error)
		}
	}

	if len(validCartItems) == 0 {
		log.Printf("No valid items in cart for user %v", userID)
		helper.ResponseWithErr(c, http.StatusBadRequest, "No valid products in cart",
			"Some products in your cart are no longer available", "")
		return
	}

	if invalidProductFound || totalPrice != cart.TotalPrice {
		cart.TotalPrice = totalPrice
		if err := database.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("cart_id = ?", cart.ID).Delete(&userModels.CartItem{}).Error; err != nil {
				return err
			}
			if err := tx.Save(&cart).Error; err != nil {
				return err
			}
			for _, item := range validCartItems {
				newItem := userModels.CartItem{
					CartID:             cart.ID,
					ProductID:          item.ProductID,
					VariantsID:         item.VariantsID,
					Quantity:           item.Quantity,
					Price:              item.Price,
					DiscountedPrice:    item.DiscountedPrice,
					OriginalPrice:      item.OriginalPrice,
					DiscountPercentage: item.DiscountPercentage,
					OfferName:          item.OfferName,
					IsOfferApplied:     item.IsOfferApplied,
					ItemTotal:          item.ItemTotal,
				}
				if err := tx.Create(&newItem).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			log.Printf("Failed to save updated cart for user %v: %v", userID, err)
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update cart", err.Error(), "")
			return
		}
		cart.CartItems = validCartItems
	}

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
				totalPrice >= appliedCoupon.MinPurchaseAmount  {
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
			"title":         "Checkout",
			"Cart":          cart,
			"Addresses":     addresses,
			"Shipping":      shipping,
			"FinalPrice":    finalPrice,
			"Subtotal":      totalPrice,
			"OriginalTotalPrice": originalTotalPrice,
			"Discount":      discount,
			"CouponApplied": couponApplied,
			"AppliedCoupon": appliedCoupon,
			"UserEmail":     user.Email,
			"UserPhone":     user.Phone,
			"RazorpayKey":   os.Getenv("RAZORPAY_KEY_ID"),
			"status":        "success",
			"UserName":      "Guest",
			"WishlistCount": 0,
			"CartCount":     0,
			"ProfileImage":  "",
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
		"title":         "Checkout",
		"Cart":          cart,
		"Addresses":     addresses,
		"Shipping":      shipping,
		"FinalPrice":    finalPrice,
		"Subtotal":      totalPrice,
		"OriginalTotalPrice": originalTotalPrice,
		"Discount":      discount,
		"CouponApplied": couponApplied,
		"AppliedCoupon": appliedCoupon,
		"UserEmail":     user.Email,
		"UserPhone":     user.Phone,
		"RazorpayKey":   os.Getenv("RAZORPAY_KEY_ID"),
		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
	})
}

func PlaceOrder(c *gin.Context) {
	userID, _ := c.Get("id")

	var req PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON: %v", err)
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	log.Printf("PlaceOrder request for user %v: address_id=%d, payment_method=%s", userID, req.AddressID, req.PaymentMethod)

	if req.PaymentMethod != "COD" && req.PaymentMethod != "ONLINE" {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid payment method", "Choose either COD or ONLINE", "")
		return
	}

	var cart userModels.Cart
	var validCartItems []userModels.CartItem
	var address userModels.Address
	var finalPrice float64
	var discount float64
	var coupon adminModels.Coupons
	orderID := generateOrderID()
	totalPrice := 0.0

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).
			Preload("CartItems.Product").
			Preload("CartItems.Variants").
			First(&cart).Error; err != nil {
			return fmt.Errorf("cart not found: %v", err)
		}

		for _, item := range cart.CartItems {
			var category adminModels.Category
			if item.Product.IsListed &&
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
				validCartItems = append(validCartItems, item)
			}
		}

		if len(validCartItems) == 0 {
			return fmt.Errorf("no valid items in cart")
		}

		cart.TotalPrice = totalPrice
		if err := tx.Where("cart_id = ?", cart.ID).Delete(&userModels.CartItem{}).Error; err != nil {
			return fmt.Errorf("failed to delete cart items: %v", err)
		}
		if err := tx.Save(&cart).Error; err != nil {
			return fmt.Errorf("failed to save cart: %v", err)
		}
		for _, item := range validCartItems {
			newItem := userModels.CartItem{
				CartID:             cart.ID,
				ProductID:          item.ProductID,
				VariantsID:         item.VariantsID,
				Quantity:           item.Quantity,
				Price:              item.Price,
				DiscountedPrice:    item.DiscountedPrice,
				OriginalPrice:      item.OriginalPrice,
				DiscountPercentage: item.DiscountPercentage,
				OfferName:          item.OfferName,
				IsOfferApplied:     item.IsOfferApplied,
				ItemTotal:          item.ItemTotal,
			}
			if err := tx.Create(&newItem).Error; err != nil {
				return fmt.Errorf("failed to create cart item: %v", err)
			}
		}

		if err := tx.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address).Error; err != nil {
			return fmt.Errorf("invalid address: %v", err)
		}

		finalPrice = cart.TotalPrice + shipping
		if cart.CouponID != 0 {
			if err := tx.First(&coupon, cart.CouponID).Error; err == nil {
				if coupon.IsActive &&
					coupon.ExpiryDate.After(time.Now()) &&
					coupon.UsedCount < coupon.UsageLimit &&
					cart.TotalPrice >= coupon.MinPurchaseAmount  {
					discount = cart.TotalPrice * (coupon.DiscountPercentage / 100)
					finalPrice -= discount
				} else {
					log.Printf("Coupon %s invalid in PlaceOrder: active=%v, expired=%v, used=%d/%d, min=%.2f, cartTotal=%.2f",
						coupon.CouponCode, coupon.IsActive, coupon.ExpiryDate.Before(time.Now()),
						coupon.UsedCount, coupon.UsageLimit, coupon.MinPurchaseAmount, cart.TotalPrice)
					cart.CouponID = 0
					if err := tx.Save(&cart).Error; err != nil {
						return fmt.Errorf("failed to reset coupon: %v", err)
					}
				}
			} else {
				log.Printf("Coupon ID %v not found in PlaceOrder: %v", cart.CouponID, err)
				cart.CouponID = 0
				if err := tx.Save(&cart).Error; err != nil {
					return fmt.Errorf("failed to reset coupon: %v", err)
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("Transaction error: %v", err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to validate order", err.Error(), "")
		return
	}

	if finalPrice < minimumOrderAmount {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Order amount too low",
			fmt.Sprintf("Order amount must be at least ₹%.2f", minimumOrderAmount), "")
		return
	}

	if req.PaymentMethod == "ONLINE" {
		log.Printf("Creating Razorpay order: userID=%v, finalPrice=%.2f, cart.TotalPrice=%.2f, discount=%.2f",
			userID, finalPrice, cart.TotalPrice, discount)

		client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))
		amount := int(finalPrice * 100)
		data := map[string]interface{}{
			"amount":   amount,
			"currency": "INR",
			"receipt":  fmt.Sprintf("receipt_%d_%d", userID, time.Now().Unix()),
		}

		razorpayOrder, err := client.Order.Create(data, nil)
		if err != nil {
			log.Printf("Razorpay error: %v", err)
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create payment order", err.Error(), "")
			return
		}
		razorpayOrderID := razorpayOrder["id"].(string)

		paymentDetails := adminModels.PaymentDetails{
			UserID:          userID.(uint), // Changed from ID to UserID
			RazorpayOrderID: razorpayOrderID,
			Amount:          finalPrice,
			Status:          "Created",
		}

		if err := database.DB.Create(&paymentDetails).Error; err != nil {
			log.Printf("Error saving payment details: %v", err)
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to store payment details", err.Error(), "")
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

	var order userModels.Orders
	err = database.DB.Transaction(func(tx *gorm.DB) error {
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

		order = userModels.Orders{
			UserID:        userID.(uint),
			OrderIdUnique: orderID,
			AddressID:     req.AddressID,
			TotalPrice:    finalPrice,
			Subtotal:      cart.TotalPrice,
			Discount:      discount,
			CouponID:      cart.CouponID,
			Status:        "Confirmed",
			PaymentMethod: req.PaymentMethod,
			OrderDate:     time.Now(),
		}

		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("failed to create order: %v", err)
		}

		for _, item := range validCartItems {
			orderItem := userModels.OrderItem{
				OrderID:    order.ID,
				ProductID:  item.ProductID,
				VariantsID: item.VariantsID,
				Quantity:   item.Quantity,
				Price:      item.DiscountedPrice,
				Status:     "Active",
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

		if err := tx.Where("cart_id = ?", cart.ID).Delete(&userModels.CartItem{}).Error; err != nil {
			return fmt.Errorf("failed to delete cart items: %v", err)
		}
		return tx.Delete(&cart).Error
	})

	if err != nil {
		log.Printf("Order creation error: %v", err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to place order", err.Error(), "")
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

type PaymentRequest struct {
	AddressID     uint   `json:"address_id" binding:"required"`
	PaymentMethod string `json:"payment_method" binding:"required"`
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
		"title":   "Order Successful",
		"OrderID": order.OrderIdUnique,
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
