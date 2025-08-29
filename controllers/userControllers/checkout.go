package controllers

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/services"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const shipping = 10.0
const minimumOrderAmount = 10.0

func ShowCheckout(c *gin.Context) {
	pkg.Log.Info("Request to checkout")
	uid, _ := c.Get("id")
	userID := uid.(uint)

	pkg.Log.Debug("Feach user ID", zap.Uint("userID", userID))

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
			item.RegularPrice = offerDetails.OriginalPrice
			item.OfferDiscountPercentage = offerDetails.DiscountPercentage
			item.OfferName = offerDetails.OfferName
			item.IsOfferApplied = offerDetails.IsOfferApplied

			itemOfferDiscount := (offerDetails.OriginalPrice - offerDetails.DiscountedPrice) * float64(item.Quantity)
			offerDiscount += itemOfferDiscount

			item.SalePrice = offerDetails.DiscountedPrice * float64(item.Quantity)
			totalPrice += item.SalePrice
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

	var Wallet userModels.Wallet
	if err := database.DB.Where("user_id = ?", userID).First(&Wallet).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Wallet not found", "Wallet not found", "")
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
			"Subtotal":           originalTotalPrice,
			"OriginalTotalPrice": originalTotalPrice,
			"TotalDiscount":      offerDiscount + couponDiscount,
			"OfferDiscount":      offerDiscount,
			"CouponDiscount":     couponDiscount,
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
			"Wallet":             Wallet,
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
		"TotalDiscount":      offerDiscount + couponDiscount,
		"OfferDiscount":      offerDiscount,
		"CouponDiscount":     couponDiscount,
		"CouponApplied":      couponApplied,
		"AppliedCoupon":      appliedCoupon,
		"UserEmail":          user.Email,
		"UserPhone":          user.Phone,
		"RazorpayKey":        os.Getenv("RAZORPAY_KEY_ID"),
		"UserName":           userNameStr,
		"ProfileImage":       userData.ProfileImage,
		"WishlistCount":      wishlistCount,
		"CartCount":          cartCount,
		"Wallet":             Wallet,
	})
}

type PaymentRequest struct {
	AddressID     uint   `json:"address_id" binding:"required"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}

func PlaceOrder(c *gin.Context) {
	userID := helper.FetchUserID(c)

	var req PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON: %v", err)
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	log.Printf("PlaceOrder request for user %v: address_id=%d, payment_method=%s", userID, req.AddressID, req.PaymentMethod)

	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		pkg.Log.Error("User not found", zap.Uint("userID", userID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "User not found", "/cart")
		return
	}

	cart, err := services.FetchCartByUserID(userID)
	if err != nil {
		pkg.Log.Warn("Cart is empty or not found", zap.Uint("userID", userID))
		helper.ResponseWithErr(c, http.StatusNotFound, "Cart is empty", "Add products to your cart", "/cart")
		return
	}

	totalPrice, originalTotalPrice, OfferDiscount, validCartItems, err := services.ValidateCartItems(cart, database.DB)
	if err != nil {
		pkg.Log.Error("Failed to validate cart items", zap.Uint("userID", userID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, err.Error(), err.Error(), "/cart")
		return
	}

	cart.OriginalTotalPrice = originalTotalPrice
	cart.TotalPrice = totalPrice
	if err := database.DB.Save(&cart).Error; err != nil {
		pkg.Log.Error("Failed to save cart", zap.Uint("userID", userID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process cart", "Something went wrong", "/cart")
		return
	}

	var address userModels.Address
	if err := database.DB.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address).Error; err != nil {
		pkg.Log.Error("Address not found", zap.Uint("addressID", req.AddressID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid address", "Address not found", "/checkout")
		return
	}

	var couponDiscount float64
	var couponCode string
	var couponID uint
	if cart.CouponID != 0 {
		var coupon adminModels.Coupons
		if err := database.DB.First(&coupon, cart.CouponID).Error; err != nil {
			pkg.Log.Warn("Coupon not found", zap.Uint("couponID", cart.CouponID), zap.Error(err))
			cart.CouponID = 0
			database.DB.Save(&cart)
		} else if coupon.IsActive && coupon.ExpiryDate.After(time.Now()) && coupon.UsedCount < coupon.UsageLimit && cart.OriginalTotalPrice >= coupon.MinPurchaseAmount {
			couponDiscount = cart.OriginalTotalPrice * (coupon.DiscountPercentage / 100) // Fixed: Use * for percentage
			couponCode = coupon.CouponCode
			couponID = coupon.ID
		} else {
			pkg.Log.Warn("Invalid coupon", zap.String("couponCode", coupon.CouponCode))
			cart.CouponID = 0
			database.DB.Save(&cart)
		}
	}
	finalPrice := totalPrice + shipping - couponDiscount
	if finalPrice < minimumOrderAmount {
		pkg.Log.Warn("Order amount too low", zap.Float64("finalPrice", finalPrice))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Order amount too low", fmt.Sprintf("Minimum order amount is %.2f", minimumOrderAmount), "/cart")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	orderID := helper.GenerateOrderID()
	shippingAddress := adminModels.ShippingAddress{
		OrderID:        orderID,
		UserID:         userID,
		Name:           address.Name,
		City:           address.City,
		AddressType:    address.AddressType,
		State:          address.State,
		Pincode:        address.Pincode,
		Phone:          address.Phone,
		AlternatePhone: address.AlternatePhone,
	}
	if err := tx.Create(&shippingAddress).Error; err != nil {
		pkg.Log.Error("Failed to create shipping address", zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create shipping address", "Something went wrong", "/checkout")
		return
	}

	order := userModels.Orders{
		UserID:             userID,
		OrderIdUnique:      orderID,
		AddressID:          req.AddressID,
		ShippingAddress:    shippingAddress,
		TotalPrice:         finalPrice,
		Subtotal:           originalTotalPrice,
		CouponDiscount:     couponDiscount,
		OfferDiscount:      OfferDiscount,
		TotalDiscount:      OfferDiscount + couponDiscount,
		CouponID:           couponID,
		CouponCode:         couponCode,
		PaymentMethod:      req.PaymentMethod,
		PaymentStatus:      "Pending",
		Status:             "Pending",
		OrderDate:          time.Now(),
		ShippingCost:       shipping,
		CancellationStatus: "None",
	}
	if err := tx.Create(&order).Error; err != nil {
		pkg.Log.Error("Failed to create order", zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create order", "Something went wrong", "/checkout")
		return
	}

	orderBackUp := userModels.OrderBackUp{
		Subtotal:      originalTotalPrice,
		ShippingCost:  shipping,
		TotalPrice:    finalPrice,
		OfferDiscount: OfferDiscount,
		OrderIdUnique: orderID,
	}
	if err := tx.Create(&orderBackUp).Error; err != nil {
		pkg.Log.Error("Failed to create order backup", zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create order backup", "Something went wrong", "/checkout")
		return
	}

	// Create order items (common for all payment methods)
	for _, item := range validCartItems {
		orderItem := userModels.OrderItem{
			OrderID:        order.ID,
			ProductID:      item.ProductID,
			VariantsID:     item.VariantsID,
			Quantity:       item.Quantity,
			UnitPrice:      item.DiscountedPrice,
			ItemTotal:      item.DiscountedPrice * float64(item.Quantity),
			DiscountAmount: (item.SalePrice - item.DiscountedPrice) * float64(item.Quantity),
			OfferName:      item.OfferName,
			Status:         "Active",
			ReturnStatus:   "None",
		}
		if err := tx.Create(&orderItem).Error; err != nil {
			pkg.Log.Error("Failed to create order item", zap.Uint("productID", item.ProductID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create order items", "Something went wrong", "/checkout")
			return
		}
	}

	// Stock check/deduction logic
	stockOk := true
	for _, item := range validCartItems {
		var variant adminModels.Variants
		if err := tx.First(&variant, item.VariantsID).Error; err != nil {
			pkg.Log.Error("Variant not found", zap.Uint("variantsID", item.VariantsID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusBadRequest, "Variant not found", "Product variant not found", "/cart")
			return
		}
		if variant.Stock < item.Quantity {
			stockOk = false
			pkg.Log.Error("Insufficient stock", zap.Uint("variantsID", item.VariantsID), zap.Uint("available", variant.Stock), zap.Uint("required", item.Quantity))
			break
		}
	}
	if !stockOk {
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusBadRequest, "Insufficient stock", "One or more products are out of stock", "/cart")
		return
	}

	if cart.CouponID != 0 {
		var coupon adminModels.Coupons
		if err := tx.First(&coupon, cart.CouponID).Error; err == nil {
			coupon.UsedCount++
			if err := tx.Save(&coupon).Error; err != nil {
				pkg.Log.Error("Failed to update coupon usage", zap.Uint("couponID", cart.CouponID), zap.Error(err))
				tx.Rollback()
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update coupon", "Something went wrong", "/checkout")
				return
			}
		}
	}

	switch req.PaymentMethod {
	case "COD":
		// Deduct stock for COD
		for _, item := range validCartItems {
			var variant adminModels.Variants
			tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&variant, item.VariantsID)
			variant.Stock -= item.Quantity
			if err := tx.Save(&variant).Error; err != nil {
				pkg.Log.Error("Failed to update variant stock", zap.Uint("variantsID", item.VariantsID), zap.Error(err))
				tx.Rollback()
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update stock", "Something went wrong", "/checkout")
				return
			}
			// Update product InStock
			var product adminModels.Product
			if err := tx.Preload("Variants").First(&product, item.ProductID).Error; err != nil {
				pkg.Log.Error("Product not found", zap.Uint("productID", item.ProductID), zap.Error(err))
				tx.Rollback()
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Product not found", "Something went wrong", "/checkout")
				return
			}
			var totalStock uint
			for _, v := range product.Variants {
				totalStock += v.Stock
			}
			product.InStock = totalStock > 0
			if err := tx.Save(&product).Error; err != nil {
				pkg.Log.Error("Failed to update product stock", zap.Uint("productID", item.ProductID), zap.Error(err))
				tx.Rollback()
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update product stock", "Something went wrong", "/checkout")
				return
			}
		}

		payment := adminModels.PaymentDetails{
			OrderID:       order.ID,
			UserID:        userID,
			AddressID:     req.AddressID,
			PaymentMethod: "COD",
			Amount:        finalPrice,
			Status:        "Pending",
			Attempts:      1,
			CreatedAt:     time.Now(),
		}
		if err := tx.Create(&payment).Error; err != nil {
			pkg.Log.Error("Failed to create payment", zap.Uint("orderID", order.ID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process COD", "Something went wrong", "/checkout")
			return
		}
		order.PaymentStatus = "Pending"
		order.Status = "Confirmed"
		if err := tx.Save(&order).Error; err != nil {
			pkg.Log.Error("Failed to update order", zap.Uint("orderID", order.ID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process COD", "Something went wrong", "/checkout")
			return
		}

		if err := services.ClearCart(userID, tx); err != nil {
			pkg.Log.Error("Failed to clear cart", zap.Uint("userID", userID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to clear cart", "Something went wrong", "/checkout")
			return
		}
		tx.Commit()
		pkg.Log.Info("COD order placed successfully", zap.String("orderID", order.OrderIdUnique), zap.Uint("userID", userID))
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"message":  "Order placed successfully",
			"order_id": order.OrderIdUnique,
			"redirect": fmt.Sprintf("/order/success?order_id=%s", order.OrderIdUnique),
		})

	case "ONLINE":
		// No stock deduction here - only checked above
		razorpayOrder, err := services.CreateRazorpayOrder(int(math.Round(finalPrice))) // Fixed: Use finalPrice
		if err != nil {
			pkg.Log.Error("Failed to create Razorpay order", zap.Float64("amount", finalPrice), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create Razorpay order", "Something went wrong", "/checkout")
			return
		}

		razorpayOrderID, ok := razorpayOrder["id"].(string)
		if !ok {
			pkg.Log.Error("Failed to extract Razorpay order ID", zap.Any("razorpayOrder", razorpayOrder))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid Razorpay response", "Something went wrong", "/checkout")
			return
		}

		order.RazorpayOrderID = razorpayOrderID
		if err := tx.Save(&order).Error; err != nil {
			pkg.Log.Error("Failed to update order with Razorpay ID", zap.String("razorpayOrderID", razorpayOrderID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process Razorpay", "Something went wrong", "/checkout")
			return
		}

		payment := adminModels.PaymentDetails{
			OrderID:         order.ID,
			UserID:          userID,
			AddressID:       req.AddressID,
			PaymentMethod:   "ONLINE",
			Amount:          finalPrice,
			Status:          "Pending",
			RazorpayOrderID: razorpayOrderID,
			Attempts:        1,
			CreatedAt:       time.Now(),
		}
		if err := tx.Create(&payment).Error; err != nil {
			pkg.Log.Error("Failed to create payment record", zap.Uint("orderID", order.ID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create payment record", "Something went wrong", "/checkout")
			return
		}

		tx.Commit()
		pkg.Log.Info("Razorpay order initiated", zap.String("razorpayOrderID", razorpayOrderID), zap.Uint("userID", userID))
		c.JSON(http.StatusOK, gin.H{
			"status":            "payment_required",
			"key_id":            os.Getenv("RAZORPAY_KEY_ID"),
			"razorpay_order_id": razorpayOrderID,
			"amount":            int(math.Round(finalPrice) * 100),
			"currency":          "INR",
			"order_id":          order.OrderIdUnique,
			"prefill": gin.H{
				"name":    user.UserName,
				"email":   user.Email,
				"contact": address.Phone,
			},
			"notes": gin.H{
				"address":  address.City,
				"user_id":  userID,
				"order_id": order.ID,
			},
		})

	case "Wallet":
		// Similar to COD: deduct stock here
		var wallet userModels.Wallet
		if err := tx.First(&wallet, "user_id = ?", userID).Error; err != nil || wallet.Balance < finalPrice { // Fixed: Use finalPrice
			pkg.Log.Error("Insufficient wallet balance or wallet not found", zap.Uint("userID", userID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusBadRequest, "Insufficient wallet balance", "Add funds to your wallet", "/cart")
			return
		}

		// Deduct stock for Wallet
		for _, item := range validCartItems {
			var variant adminModels.Variants
			tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&variant, item.VariantsID)
			variant.Stock -= item.Quantity
			if err := tx.Save(&variant).Error; err != nil {
				pkg.Log.Error("Failed to update variant stock", zap.Uint("variantsID", item.VariantsID), zap.Error(err))
				tx.Rollback()
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update stock", "Something went wrong", "/checkout")
				return
			}
			// Update product InStock
			var product adminModels.Product
			if err := tx.Preload("Variants").First(&product, item.ProductID).Error; err != nil {
				pkg.Log.Error("Product not found", zap.Uint("productID", item.ProductID), zap.Error(err))
				tx.Rollback()
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Product not found", "Something went wrong", "/checkout")
				return
			}
			var totalStock uint
			for _, v := range product.Variants {
				totalStock += v.Stock
			}
			product.InStock = totalStock > 0
			if err := tx.Save(&product).Error; err != nil {
				pkg.Log.Error("Failed to update product stock", zap.Uint("productID", item.ProductID), zap.Error(err))
				tx.Rollback()
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update product stock", "Something went wrong", "/checkout")
				return
			}
		}

		order.PaymentStatus = "Completed"
		order.Status = "Confirmed"
		if err := tx.Save(&order).Error; err != nil {
			pkg.Log.Error("Failed to update order", zap.Uint("orderID", order.ID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process wallet payment", "Something went wrong", "/checkout")
			return
		}
		transactionID := fmt.Sprintf("TXN-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
		payment := adminModels.PaymentDetails{
			OrderID:       order.ID,
			UserID:        userID,
			PaymentMethod: "Wallet",
			Amount:        finalPrice,
			Status:        "Completed",
			Attempts:      1,
			CreatedAt:     time.Now(),
			TransactionID: transactionID,
		}
		if err := tx.Create(&payment).Error; err != nil {
			pkg.Log.Error("Failed to create payment", zap.Uint("orderID", order.ID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process wallet payment", "Something went wrong", "/checkout")
			return
		}

		lastBalance := wallet.Balance
		wallet.Balance -= finalPrice
		if err := tx.Save(&wallet).Error; err != nil {
			pkg.Log.Error("Failed to update wallet balance", zap.Uint("userID", userID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update wallet", "Something went wrong", "/checkout")
			return
		}

		walletTransaction := userModels.WalletTransaction{
			UserID:        userID,
			WalletID:      wallet.ID,
			Amount:        finalPrice,
			LastBalance:   lastBalance,
			Description:   "Product Purchase ORD ID: " + order.OrderIdUnique,
			Type:          "Debited",
			Receipt:       "rcpt-" + uuid.New().String(),
			OrderID:       order.OrderIdUnique,
			TransactionID: transactionID,
			PaymentMethod: "Wallet",
		}
		if err := tx.Create(&walletTransaction).Error; err != nil {
			pkg.Log.Error("Failed to create wallet transaction", zap.Uint("userID", userID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to record wallet transaction", "Something went wrong", "/checkout")
			return
		}

		if err := services.ClearCart(userID, tx); err != nil {
			pkg.Log.Error("Failed to clear cart", zap.Uint("userID", userID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to clear cart", "Something went wrong", "/checkout")
			return
		}

		tx.Commit()
		pkg.Log.Info("Wallet order placed successfully", zap.String("orderID", order.OrderIdUnique), zap.Uint("userID", userID))
		c.Redirect(http.StatusFound, "/order/success?order_id="+order.OrderIdUnique)

	default:
		pkg.Log.Warn("Invalid payment method", zap.String("method", req.PaymentMethod))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid payment method", "Invalid payment method", "/checkout")
		return
	}
}

type PaymentVerification struct {
	RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
	RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
	RazorpaySignature string `json:"razorpay_signature" binding:"required"`
}

func VerifyPayment(c *gin.Context) {
	userID := helper.FetchUserID(c)

	var req PaymentVerification
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Log.Error("Failed to bind verification request", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", "Invalid request data", "/order/failure")
		return
	}

	if err := services.VerifyRazorpaySignature(req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature); err != nil {
		pkg.Log.Error("Razorpay signature verification failed", zap.String("razorpayOrderID", req.RazorpayOrderID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Payment verification failed", "Invalid payment", "/order/failure")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var order userModels.Orders
	if err := tx.Preload("OrderItems").First(&order, "razorpay_order_id = ? AND user_id = ?", req.RazorpayOrderID, userID).Error; err != nil {
		pkg.Log.Error("Order not found", zap.String("razorpayOrderID", req.RazorpayOrderID), zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusNotFound, "Order not found", "Something went wrong", "/order/failure")
		return
	}

	var payment adminModels.PaymentDetails
	if err := tx.First(&payment, "razorpay_order_id = ? AND user_id = ?", req.RazorpayOrderID, userID).Error; err != nil {
		pkg.Log.Error("Payment record not found", zap.String("razorpayOrderID", req.RazorpayOrderID), zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusNotFound, "Payment record not found", "Something went wrong", "/order/failure")
		return
	}

	// Check and deduct stock (for ONLINE only)
	stockOk := true
	for _, item := range order.OrderItems {
		var variant adminModels.Variants
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&variant, item.VariantsID).Error; err != nil {
			pkg.Log.Error("Variant not found during verification", zap.Uint("variantsID", item.VariantsID), zap.Error(err))
			stockOk = false
			break
		}
		if variant.Stock < item.Quantity {
			pkg.Log.Error("Insufficient stock during verification", zap.Uint("variantsID", item.VariantsID), zap.Uint("available", variant.Stock), zap.Uint("required", item.Quantity))
			stockOk = false
			break
		}
		variant.Stock -= item.Quantity
		if err := tx.Save(&variant).Error; err != nil {
			pkg.Log.Error("Failed to update variant stock during verification", zap.Uint("variantsID", item.VariantsID), zap.Error(err))
			stockOk = false
			break
		}
		// Update product InStock
		var product adminModels.Product
		if err := tx.Preload("Variants").First(&product, item.ProductID).Error; err != nil {
			pkg.Log.Error("Product not found during verification", zap.Uint("productID", item.ProductID), zap.Error(err))
			stockOk = false
			break
		}
		var totalStock uint
		for _, v := range product.Variants {
			totalStock += v.Stock
		}
		product.InStock = totalStock > 0
		if err := tx.Save(&product).Error; err != nil {
			pkg.Log.Error("Failed to update product stock during verification", zap.Uint("productID", item.ProductID), zap.Error(err))
			stockOk = false
			break
		}
	}

	if !stockOk {
		// Initiate refund and fail order
		// if err := services.RefundRazorpayPayment(req.RazorpayPaymentID, order.TotalPrice); err != nil {
		//     // Log but continue to fail order (refund might need manual handling)
		//     pkg.Log.Warn("Refund failed, manual intervention needed", zap.String("paymentID", req.RazorpayPaymentID), zap.Error(err))
		// }
		// payment.Status = "Refunded"
		payment.FailureReason = "Insufficient stock"
		tx.Save(&payment)
		order.Status = "Failed"
		// order.PaymentStatus = "Refunded"
		order.OrderError = "Insufficient stock after payment"
		tx.Save(&order)
		tx.Commit()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Insufficient stock", "Order failed due to stock unavailability. Refund initiated.", "/order/failure")
		return
	}

	payment.RazorpayPaymentID = req.RazorpayPaymentID
	payment.RazorpaySignature = req.RazorpaySignature
	payment.Status = "Paid"
	payment.Attempts++
	if err := tx.Save(&payment).Error; err != nil {
		pkg.Log.Error("Failed to update payment status", zap.Uint("orderID", order.ID), zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update payment", "Something went wrong", "/order/failure")
		return
	}

	order.PaymentStatus = "Paid"
	order.Status = "Confirmed"
	if err := tx.Save(&order).Error; err != nil {
		pkg.Log.Error("Failed to update order", zap.String("orderID", order.OrderIdUnique), zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update order", "Something went wrong", "/order/failure")
		return
	}

	if err := services.ClearCart(userID, tx); err != nil {
		pkg.Log.Error("Failed to clear cart", zap.Uint("userID", userID), zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to clear cart", "Something went wrong", "/order/failure")
		return
	}

	tx.Commit()
	pkg.Log.Info("Razorpay payment verified successfully", zap.String("orderID", order.OrderIdUnique), zap.Uint("userID", userID))
	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "Payment verified successfully",
		"order_id": order.OrderIdUnique,
		"redirect": fmt.Sprintf("/order/success?order_id=%s", order.OrderIdUnique),
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
	pkg.Log.Info("Rendering order success page")

	orderID := c.Query("order_id")
	var order userModels.Orders
	if err := database.DB.Preload("OrderItems").Preload("OrderItems.Product").Preload("OrderItems.Variants").Preload("ShippingAddress").First(&order, "order_id_unique = ?", orderID).Error; err != nil {
		pkg.Log.Error("Order not found", zap.String("orderID", orderID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Order not found", "Something went wrong", "/cart")
		return
	}

	pkg.Log.Info("Order success page loaded", zap.String("orderID", orderID))
	c.HTML(http.StatusOK, "orderSuccess.html", gin.H{
		"status":        "Success",
		"message":       "Order placed successfully",
		"OrderID":       order.OrderIdUnique,
		"PaymentMethod": order.PaymentMethod,
		"OrderDate":     order.OrderDate.Format("January 2, 2006"),
		"ExpectedDate":  order.OrderDate.AddDate(0, 0, 7).Format("January 2, 2006"),
		"Order":         order,
		"code":          http.StatusOK,
	})
}

func ShowOrderFailure(c *gin.Context) {
	pkg.Log.Info("Rendering order failure page")
	errorMsg := c.Query("error")
	if errorMsg == "" {
		errorMsg = "Order placement failed"
	}

	orderID := c.Query("order_id")
	if orderID == "" {
		log.Println("order id is empty")
	}

	var order userModels.Orders
	if err := database.DB.Where("order_id_unique = ?", orderID).First(&order).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "not found order id", "", "")
		return
	}

	order.Status = "Failed"
	order.PaymentStatus = "Pending"

	if err := database.DB.Save(&order).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "error saving orders", "", "")
		return
	}

	var orderItems []userModels.OrderItem
	if err := database.DB.Where("order_id = ?", order.ID).Preload("Product").Preload("Variants").Find(&orderItems).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "order item is not found", "", "")
		return
	}

	var shipping adminModels.ShippingAddress
	if err := database.DB.Where("order_id = ?", orderID).First(&shipping).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "address not found", "", "")
		return
	}

	c.HTML(http.StatusOK, "orderFailure.html", gin.H{
		"status":     "Failed",
		"orderItems": orderItems,
		"message":    errorMsg,
		"address":    shipping,
		"code":       http.StatusOK,
		"ordID":      orderID,
	})
}
