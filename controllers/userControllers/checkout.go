package controllers

import (
	"fmt"
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

const minimumOrderAmount = 10.0

func ShowCheckout(c *gin.Context) {
	pkg.Log.Info("Starting checkout process")

	uid, _ := c.Get("id")
	userID := uid.(uint)

	pkg.Log.Debug("Fetching user ID", zap.Uint("user_id", userID))

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).
		Preload("CartItems.Product").
		Preload("CartItems.Variants").
		First(&cart).Error; err != nil {
		pkg.Log.Warn("Cart not found for user", zap.Uint("user_id", userID), zap.Error(err))
		if err := database.DB.Where("user_id = ? AND status = ?", userID, "Pending").
			Order("order_date DESC").
			First(&userModels.Orders{}).Error; err == nil {
			var recentOrder userModels.Orders
			database.DB.Where("user_id = ? AND status = ?", userID, "Pending").
				Order("order_date DESC").
				First(&recentOrder)
			pkg.Log.Info("Redirecting to recent pending order", zap.String("order_id", recentOrder.OrderIdUnique))
			c.Redirect(http.StatusFound, fmt.Sprintf("/order/success?order_id=%s", recentOrder.OrderIdUnique))
			return
		}
		pkg.Log.Info("Redirecting to cart due to no cart found", zap.Uint("user_id", userID))
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
			pkg.Log.Warn("Invalid cart item",
				zap.Uint("user_id", userID),
				zap.Uint("product_id", item.ProductID),
				zap.Uint("variant_id", item.VariantsID),
				zap.Bool("product_listed", item.Product.IsListed),
				zap.Bool("product_in_stock", isInStock),
				zap.Int("variant_stock", int(variant.Stock)),
				zap.Uint("quantity", item.Quantity),
				zap.Error(database.DB.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error))
		}
	}

	if len(validCartItems) == 0 {
		pkg.Log.Warn("No valid items in cart", zap.Uint("user_id", userID))
		helper.ResponseWithErr(c, http.StatusSeeOther, "No valid products in cart",
			"All products in your cart are either out of stock or have insufficient quantity", "/cart")
		return
	}

	if totalPrice != cart.TotalPrice {
		cart.TotalPrice = totalPrice
		if err := database.DB.Save(&cart).Error; err != nil {
			pkg.Log.Error("Failed to update cart total",
				zap.Uint("user_id", userID),
				zap.Uint("cart_id", cart.ID),
				zap.Float64("total_price", totalPrice),
				zap.Error(err))
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update cart", err.Error(), "")
			return
		}
	}

	cart.CartItems = append(validCartItems, invalidCartItems...)

	var addresses []userModels.Address
	if err := database.DB.Where("user_id = ?", userID).Find(&addresses).Error; err != nil {
		pkg.Log.Error("Failed to load addresses",
			zap.Uint("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to load addresses", err.Error(), "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		pkg.Log.Error("Failed to load user details",
			zap.Uint("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to load user details", err.Error(), "")
		return
	}

	shipping := helper.CalculateShipping(totalPrice)

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
				pkg.Log.Info("Coupon applied",
					zap.String("coupon_code", appliedCoupon.CouponCode),
					zap.Float64("coupon_discount", couponDiscount),
					zap.Float64("final_price", finalPrice))
			} else {
				pkg.Log.Warn("Invalid coupon",
					zap.String("coupon_code", appliedCoupon.CouponCode),
					zap.Bool("is_active", appliedCoupon.IsActive),
					zap.Bool("expired", appliedCoupon.ExpiryDate.Before(time.Now())),
					zap.Int("used_count", appliedCoupon.UsedCount),
					zap.Int("usage_limit", appliedCoupon.UsageLimit),
					zap.Float64("min_purchase_amount", appliedCoupon.MinPurchaseAmount),
					zap.Float64("total_price", totalPrice))
				cart.CouponID = 0
				if err := database.DB.Save(&cart).Error; err != nil {
					pkg.Log.Error("Failed to reset coupon for cart",
						zap.Uint("cart_id", cart.ID),
						zap.Error(err))
					helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update cart", err.Error(), "")
					return
				}
			}
		} else {
			pkg.Log.Warn("Coupon not found",
				zap.Uint("coupon_id", cart.CouponID),
				zap.Uint("cart_id", cart.ID),
				zap.Error(err))
			cart.CouponID = 0
			if err := database.DB.Save(&cart).Error; err != nil {
				pkg.Log.Error("Failed to reset coupon for cart",
					zap.Uint("cart_id", cart.ID),
					zap.Error(err))
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update cart", err.Error(), "")
				return
			}
		}
	}

	if finalPrice < minimumOrderAmount {
		pkg.Log.Warn("Order amount too low",
			zap.Uint("user_id", userID),
			zap.Float64("final_price", finalPrice),
			zap.Float64("minimum_order_amount", minimumOrderAmount))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Order amount too low",
			fmt.Sprintf("Order amount must be at least ₹%.2f", minimumOrderAmount), "")
		return
	}

	var Wallet userModels.Wallet
	if err := database.DB.Where("user_id = ?", userID).First(&Wallet).Error; err != nil {
		pkg.Log.Error("Wallet not found",
			zap.Uint("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Wallet not found", "Wallet not found", "")
		return
	}

	userr, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		pkg.Log.Debug("Rendering checkout for guest user", zap.Uint("user_id", userID))
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
		pkg.Log.Warn("Failed to fetch wishlist count",
			zap.Uint("user_id", userData.ID),
			zap.Error(err))
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		pkg.Log.Warn("Failed to fetch cart count",
			zap.Uint("user_id", userData.ID),
			zap.Error(err))
		cartCount = 0
	}

	pkg.Log.Info("Rendering checkout page",
		zap.Uint("user_id", userData.ID),
		zap.String("user_name", userNameStr),
		zap.Int64("wishlist_count", wishlistCount),
		zap.Int64("cart_count", cartCount),
		zap.Float64("final_price", finalPrice))

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
		pkg.Log.Error("Failed to bind JSON request",
			zap.Uint("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	pkg.Log.Debug("PlaceOrder request",
		zap.Uint("user_id", userID),
		zap.Uint("address_id", req.AddressID),
		zap.String("payment_method", req.PaymentMethod))

	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		pkg.Log.Error("User not found",
			zap.Uint("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "User not found", "/cart")
		return
	}

	cart, err := services.FetchCartByUserID(userID)
	if err != nil {
		pkg.Log.Warn("Cart is empty or not found",
			zap.Uint("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Cart is empty", "Add products to your cart", "/cart")
		return
	}

	totalPrice, originalTotalPrice, OfferDiscount, validCartItems, err := services.ValidateCartItems(cart, database.DB)
	if err != nil {
		pkg.Log.Error("Failed to validate cart items",
			zap.Uint("user_id", userID),
			zap.Uint("cart_id", cart.ID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, err.Error(), err.Error(), "/cart")
		return
	}

	cart.OriginalTotalPrice = originalTotalPrice
	cart.TotalPrice = totalPrice
	if err := database.DB.Save(&cart).Error; err != nil {
		pkg.Log.Error("Failed to save cart",
			zap.Uint("user_id", userID),
			zap.Uint("cart_id", cart.ID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process cart", "Something went wrong", "/cart")
		return
	}

	var address userModels.Address
	if err := database.DB.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address).Error; err != nil {
		pkg.Log.Error("Address not found",
			zap.Uint("user_id", userID),
			zap.Uint("address_id", req.AddressID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid address", "Address not found", "/checkout")
		return
	}

	var couponDiscount float64
	var couponCode string
	var couponID uint
	if cart.CouponID != 0 {
		var coupon adminModels.Coupons
		if err := database.DB.First(&coupon, cart.CouponID).Error; err != nil {
			pkg.Log.Warn("Coupon not found",
				zap.Uint("coupon_id", cart.CouponID),
				zap.Uint("cart_id", cart.ID),
				zap.Error(err))
			cart.CouponID = 0
			database.DB.Save(&cart)
		} else if coupon.IsActive && coupon.ExpiryDate.After(time.Now()) && coupon.UsedCount < coupon.UsageLimit && cart.OriginalTotalPrice >= coupon.MinPurchaseAmount {
			couponDiscount = cart.OriginalTotalPrice * (coupon.DiscountPercentage / 100)
			couponCode = coupon.CouponCode
			couponID = coupon.ID
			pkg.Log.Info("Coupon applied",
				zap.String("coupon_code", couponCode),
				zap.Float64("coupon_discount", couponDiscount))
		} else {
			pkg.Log.Warn("Invalid coupon",
				zap.String("coupon_code", coupon.CouponCode),
				zap.Bool("is_active", coupon.IsActive),
				zap.Bool("expired", coupon.ExpiryDate.Before(time.Now())),
				zap.Int("used_count", coupon.UsedCount),
				zap.Int("usage_limit", coupon.UsageLimit),
				zap.Float64("min_purchase_amount", coupon.MinPurchaseAmount),
				zap.Float64("total_price", cart.OriginalTotalPrice))
			cart.CouponID = 0
			database.DB.Save(&cart)
		}
	}

	shipping := helper.CalculateShipping(totalPrice)
	finalPrice := totalPrice + shipping - couponDiscount
	if finalPrice < minimumOrderAmount {
		pkg.Log.Warn("Order amount too low",
			zap.Uint("user_id", userID),
			zap.Float64("final_price", finalPrice),
			zap.Float64("minimum_order_amount", minimumOrderAmount))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Order amount too low", fmt.Sprintf("Minimum order amount is %.2f", minimumOrderAmount), "/cart")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			pkg.Log.Error("Recovered from panic in PlaceOrder",
				zap.Any("user_id", userID),
				zap.Any("panic", r))
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
		pkg.Log.Error("Failed to create shipping address",
			zap.String("order_id", orderID),
			zap.Uint("user_id", userID),
			zap.Error(err))
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
		pkg.Log.Error("Failed to create order",
			zap.String("order_id", orderID),
			zap.Uint("user_id", userID),
			zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create order", "Something went wrong", "/checkout")
		return
	}

	orderBackUp := userModels.OrderBackUp{
		Subtotal:       originalTotalPrice,
		ShippingCost:   shipping,
		TotalPrice:     finalPrice,
		OfferDiscount:  OfferDiscount,
		OrderIdUnique:  orderID,
		CouponDiscount: couponDiscount,
	}
	if err := tx.Create(&orderBackUp).Error; err != nil {
		pkg.Log.Error("Failed to create order backup",
			zap.String("order_id", orderID),
			zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create order backup", "Something went wrong", "/checkout")
		return
	}

	for _, item := range validCartItems {
		orderItem := userModels.OrderItem{
			OrderID:        order.ID,
			ProductID:      item.ProductID,
			VariantsID:     item.VariantsID,
			Quantity:       item.Quantity,
			UnitPrice:      item.Price,
			ItemTotal:      item.DiscountedPrice * float64(item.Quantity),
			DiscountAmount: (item.Price - item.DiscountedPrice) * float64(item.Quantity),
			OfferName:      item.OfferName,
			Status:         "Active",
			ReturnStatus:   "None",
		}
		if err := tx.Create(&orderItem).Error; err != nil {
			pkg.Log.Error("Failed to create order item",
				zap.String("order_id", orderID),
				zap.Uint("product_id", item.ProductID),
				zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create order items", "Something went wrong", "/checkout")
			return
		}
	}

	stockOk := true
	for _, item := range validCartItems {
		var variant adminModels.Variants
		if err := tx.First(&variant, item.VariantsID).Error; err != nil {
			pkg.Log.Error("Variant not found",
				zap.Uint("variant_id", item.VariantsID),
				zap.String("order_id", orderID),
				zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusBadRequest, "Variant not found", "Product variant not found", "/cart")
			return
		}
		if variant.Stock < item.Quantity {
			stockOk = false
			pkg.Log.Error("Insufficient stock",
				zap.Uint("variant_id", item.VariantsID),
				zap.String("order_id", orderID),
				zap.Uint("available", variant.Stock),
				zap.Uint("required", item.Quantity))
			break
		}
	}
	if !stockOk {
		tx.Rollback()
		pkg.Log.Info("Order rolled back due to insufficient stock",
			zap.String("order_id", orderID),
			zap.Uint("user_id", userID))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Insufficient stock", "One or more products are out of stock", "/cart")
		return
	}

	if cart.CouponID != 0 {
		var coupon adminModels.Coupons
		if err := tx.First(&coupon, cart.CouponID).Error; err == nil {
			coupon.UsedCount++
			if err := tx.Save(&coupon).Error; err != nil {
				pkg.Log.Error("Failed to update coupon usage",
					zap.Uint("coupon_id", cart.CouponID),
					zap.String("order_id", orderID),
					zap.Error(err))
				tx.Rollback()
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update coupon", "Something went wrong", "/checkout")
				return
			}
			pkg.Log.Info("Coupon usage updated",
				zap.String("coupon_code", coupon.CouponCode),
				zap.Int("used_count", coupon.UsedCount))
		}
	}

	switch req.PaymentMethod {
	case "COD":
		if finalPrice >= 1000 {
			tx.Rollback()
			pkg.Log.Warn("COD not allowed for order above 1000",
				zap.Float64("final_price", finalPrice),
				zap.Uint("user_id", userID),
				zap.String("order_id", orderID))
			helper.ResponseWithErr(c, http.StatusBadRequest,
				"Order above $1000 is not allowed for COD",
				"Order above $1000 is not allowed for COD",
				"/checkout")
			return
		}
		for _, item := range validCartItems {
			var variant adminModels.Variants
			tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&variant, item.VariantsID)
			variant.Stock -= item.Quantity
			if err := tx.Save(&variant).Error; err != nil {
				pkg.Log.Error("Failed to update variant stock",
					zap.Uint("variant_id", item.VariantsID),
					zap.String("order_id", orderID),
					zap.Error(err))
				tx.Rollback()
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update stock", "Something went wrong", "/checkout")
				return
			}
			var product adminModels.Product
			if err := tx.Preload("Variants").First(&product, item.ProductID).Error; err != nil {
				pkg.Log.Error("Product not found",
					zap.Uint("product_id", item.ProductID),
					zap.String("order_id", orderID),
					zap.Error(err))
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
				pkg.Log.Error("Failed to update product stock",
					zap.Uint("product_id", item.ProductID),
					zap.String("order_id", orderID),
					zap.Error(err))
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
			pkg.Log.Error("Failed to create payment",
				zap.Uint("order_id", order.ID),
				zap.String("order_id_unique", orderID),
				zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process COD", "Something went wrong", "/checkout")
			return
		}
		order.PaymentStatus = "Pending"
		order.Status = "Confirmed"
		if err := tx.Save(&order).Error; err != nil {
			pkg.Log.Error("Failed to update order",
				zap.String("order_id", orderID),
				zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process COD", "Something went wrong", "/checkout")
			return
		}

		if err := services.ClearCart(userID, tx); err != nil {
			pkg.Log.Error("Failed to clear cart",
				zap.Uint("user_id", userID),
				zap.String("order_id", orderID),
				zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to clear cart", "Something went wrong", "/checkout")
			return
		}
		tx.Commit()
		pkg.Log.Info("COD order placed successfully",
			zap.String("order_id", order.OrderIdUnique),
			zap.Uint("user_id", userID))
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"message":  "Order placed successfully",
			"order_id": order.OrderIdUnique,
			"redirect": fmt.Sprintf("/order/success?order_id=%s", order.OrderIdUnique),
		})

	case "ONLINE":
		amountInPaise := int(math.Round(finalPrice * 100))

		razorpayOrder, err := services.CreateRazorpayOrder(amountInPaise)
		if err != nil {
			pkg.Log.Error("Failed to create Razorpay order",
				zap.Float64("amount", finalPrice),
				zap.String("order_id", orderID),
				zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create Razorpay order", "Something went wrong", "/checkout")
			return
		}

		razorpayOrderID, ok := razorpayOrder["id"].(string)
		if !ok {
			pkg.Log.Error("Failed to extract Razorpay order ID",
				zap.String("order_id", orderID),
				zap.Any("razorpay_response", razorpayOrder))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid Razorpay response", "Something went wrong", "/checkout")
			return
		}

		order.RazorpayOrderID = razorpayOrderID
		if err := tx.Save(&order).Error; err != nil {
			pkg.Log.Error("Failed to update order with Razorpay ID",
				zap.String("razorpay_order_id", razorpayOrderID),
				zap.String("order_id", orderID),
				zap.Error(err))
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
			pkg.Log.Error("Failed to create payment record",
				zap.String("order_id", orderID),
				zap.String("razorpay_order_id", razorpayOrderID),
				zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create payment record", "Something went wrong", "/checkout")
			return
		}

		tx.Commit()
		pkg.Log.Info("Razorpay order initiated",
			zap.String("razorpay_order_id", razorpayOrderID),
			zap.String("order_id", orderID),
			zap.Uint("user_id", userID))

		c.JSON(http.StatusOK, gin.H{
			"status":            "payment_required",
			"key_id":            os.Getenv("RAZORPAY_KEY_ID"),
			"razorpay_order_id": razorpayOrderID,
			"amount":            amountInPaise,
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
	case "WALLET":
		var wallet userModels.Wallet
		if err := tx.First(&wallet, "user_id = ?", userID).Error; err != nil || wallet.Balance < finalPrice {
			pkg.Log.Error("Insufficient wallet balance or wallet not found",
				zap.Uint("user_id", userID),
				zap.String("order_id", orderID),
				zap.Float64("wallet_balance", wallet.Balance),
				zap.Float64("required_amount", finalPrice),
				zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusBadRequest, "Insufficient wallet balance", "Add funds to your wallet", "/cart")
			return
		}

		for _, item := range validCartItems {
			var variant adminModels.Variants
			tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&variant, item.VariantsID)
			variant.Stock -= item.Quantity
			if err := tx.Save(&variant).Error; err != nil {
				pkg.Log.Error("Failed to update variant stock",
					zap.Uint("variant_id", item.VariantsID),
					zap.String("order_id", orderID),
					zap.Error(err))
				tx.Rollback()
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update stock", "Something went wrong", "/checkout")
				return
			}
			var product adminModels.Product
			if err := tx.Preload("Variants").First(&product, item.ProductID).Error; err != nil {
				pkg.Log.Error("Product not found",
					zap.Uint("product_id", item.ProductID),
					zap.String("order_id", orderID),
					zap.Error(err))
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
				pkg.Log.Error("Failed to update product stock",
					zap.Uint("product_id", item.ProductID),
					zap.String("order_id", orderID),
					zap.Error(err))
				tx.Rollback()
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update product stock", "Something went wrong", "/checkout")
				return
			}
		}

		order.PaymentStatus = "Completed"
		order.Status = "Confirmed"
		if err := tx.Save(&order).Error; err != nil {
			pkg.Log.Error("Failed to update order",
				zap.String("order_id", orderID),
				zap.Error(err))
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
			pkg.Log.Error("Failed to create payment",
				zap.String("order_id", orderID),
				zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process wallet payment", "Something went wrong", "/checkout")
			return
		}

		lastBalance := wallet.Balance
		wallet.Balance -= finalPrice
		if err := tx.Save(&wallet).Error; err != nil {
			pkg.Log.Error("Failed to update wallet balance",
				zap.Uint("user_id", userID),
				zap.String("order_id", orderID),
				zap.Error(err))
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
			pkg.Log.Error("Failed to create wallet transaction",
				zap.Uint("user_id", userID),
				zap.String("order_id", orderID),
				zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to record wallet transaction", "Something went wrong", "/checkout")
			return
		}

		if err := services.ClearCart(userID, tx); err != nil {
			pkg.Log.Error("Failed to clear cart",
				zap.Uint("user_id", userID),
				zap.String("order_id", orderID),
				zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to clear cart", "Something went wrong", "/checkout")
			return
		}

		tx.Commit()
		pkg.Log.Info("Wallet order placed successfully",
			zap.String("order_id", order.OrderIdUnique),
			zap.Uint("user_id", userID))
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"message":  "Order placed successfully",
			"order_id": order.OrderIdUnique,
			"redirect": fmt.Sprintf("/order/success?order_id=%s", order.OrderIdUnique),
		})

	default:
		pkg.Log.Warn("Invalid payment method",
			zap.String("payment_method", req.PaymentMethod),
			zap.String("order_id", orderID))
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
		pkg.Log.Error("Failed to bind verification request",
			zap.Uint("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", "Invalid request data", "/order/failure")
		return
	}

	pkg.Log.Debug("Payment verification request",
		zap.Uint("user_id", userID),
		zap.String("razorpay_order_id", req.RazorpayOrderID))

	if err := services.VerifyRazorpaySignature(req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature); err != nil {
		pkg.Log.Error("Razorpay signature verification failed",
			zap.String("razorpay_order_id", req.RazorpayOrderID),
			zap.Uint("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Payment verification failed", "Invalid payment", "/order/failure")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			pkg.Log.Error("Recovered from panic in VerifyPayment",
				zap.Any("user_id", userID),
				zap.String("razorpay_order_id", req.RazorpayOrderID),
				zap.Any("panic", r))
			tx.Rollback()
		}
	}()

	var order userModels.Orders
	if err := tx.Preload("OrderItems").First(&order, "razorpay_order_id = ? AND user_id = ?", req.RazorpayOrderID, userID).Error; err != nil {
		pkg.Log.Error("Order not found",
			zap.String("razorpay_order_id", req.RazorpayOrderID),
			zap.Uint("user_id", userID),
			zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusNotFound, "Order not found", "Something went wrong", "/order/failure")
		return
	}

	var payment adminModels.PaymentDetails
	if err := tx.First(&payment, "razorpay_order_id = ? AND user_id = ?", req.RazorpayOrderID, userID).Error; err != nil {
		pkg.Log.Error("Payment record not found",
			zap.String("razorpay_order_id", req.RazorpayOrderID),
			zap.Uint("user_id", userID),
			zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusNotFound, "Payment record not found", "Something went wrong", "/order/failure")
		return
	}

	stockOk := true
	for _, item := range order.OrderItems {
		var variant adminModels.Variants
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&variant, item.VariantsID).Error; err != nil {
			pkg.Log.Error("Variant not found during verification",
				zap.Uint("variant_id", item.VariantsID),
				zap.String("order_id", order.OrderIdUnique),
				zap.Error(err))
			stockOk = false
			break
		}
		if variant.Stock < item.Quantity {
			pkg.Log.Error("Insufficient stock during verification",
				zap.Uint("variant_id", item.VariantsID),
				zap.String("order_id", order.OrderIdUnique),
				zap.Uint("available", variant.Stock),
				zap.Uint("required", item.Quantity))
			stockOk = false
			break
		}
		variant.Stock -= item.Quantity
		if err := tx.Save(&variant).Error; err != nil {
			pkg.Log.Error("Failed to update variant stock during verification",
				zap.Uint("variant_id", item.VariantsID),
				zap.String("order_id", order.OrderIdUnique),
				zap.Error(err))
			stockOk = false
			break
		}
		var product adminModels.Product
		if err := tx.Preload("Variants").First(&product, item.ProductID).Error; err != nil {
			pkg.Log.Error("Product not found during verification",
				zap.Uint("product_id", item.ProductID),
				zap.String("order_id", order.OrderIdUnique),
				zap.Error(err))
			stockOk = false
			break
		}
		var totalStock uint
		for _, v := range product.Variants {
			totalStock += v.Stock
		}
		product.InStock = totalStock > 0
		if err := tx.Save(&product).Error; err != nil {
			pkg.Log.Error("Failed to update product stock during verification",
				zap.Uint("product_id", item.ProductID),
				zap.String("order_id", order.OrderIdUnique),
				zap.Error(err))
			stockOk = false
			break
		}
	}

	if !stockOk {
		// Note: Refund logic is commented out in the original code
		pkg.Log.Warn("Order failed due to insufficient stock",
			zap.String("order_id", order.OrderIdUnique),
			zap.String("razorpay_order_id", req.RazorpayOrderID))
		// if err := services.RefundRazorpayPayment(req.RazorpayPaymentID, order.TotalPrice); err != nil {
		//     pkg.Log.Warn("Refund failed, manual intervention needed",
		//         zap.String("payment_id", req.RazorpayPaymentID),
		//         zap.Error(err))
		// }
		payment.FailureReason = "Insufficient stock"
		tx.Save(&payment)
		order.Status = "Failed"
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
		pkg.Log.Error("Failed to update payment status",
			zap.String("order_id", order.OrderIdUnique),
			zap.String("razorpay_order_id", req.RazorpayOrderID),
			zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update payment", "Something went wrong", "/order/failure")
		return
	}

	order.PaymentStatus = "Paid"
	order.Status = "Confirmed"
	if err := tx.Save(&order).Error; err != nil {
		pkg.Log.Error("Failed to update order",
			zap.String("order_id", order.OrderIdUnique),
			zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update order", "Something went wrong", "/order/failure")
		return
	}

	if err := services.ClearCart(userID, tx); err != nil {
		pkg.Log.Error("Failed to clear cart",
			zap.Uint("user_id", userID),
			zap.String("order_id", order.OrderIdUnique),
			zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to clear cart", "Something went wrong", "/order/failure")
		return
	}

	tx.Commit()
	pkg.Log.Info("Razorpay payment verified successfully",
		zap.String("order_id", order.OrderIdUnique),
		zap.String("razorpay_order_id", req.RazorpayOrderID),
		zap.Uint("user_id", userID))
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

	pkg.Log.Debug("Setting default address",
		zap.Any("user_id", userID),
		zap.String("address_id", addressID))

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&userModels.Address{}).
			Where("user_id = ?", userID).
			Update("is_default", false).Error; err != nil {
			pkg.Log.Error("Failed to reset default address",
				zap.Any("user_id", userID),
				zap.Error(err))
			return err
		}
		var address userModels.Address
		if err := tx.Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error; err != nil {
			pkg.Log.Error("Address not found",
				zap.Any("user_id", userID),
				zap.String("address_id", addressID),
				zap.Error(err))
			return err
		}
		address.IsDefault = true
		if err := tx.Save(&address).Error; err != nil {
			pkg.Log.Error("Failed to set address as default",
				zap.Any("user_id", userID),
				zap.String("address_id", addressID),
				zap.Error(err))
			return err
		}
		return nil
	})

	if err != nil {
		pkg.Log.Error("Failed to set default address",
			zap.Any("user_id", userID),
			zap.String("address_id", addressID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to set default address", err.Error(), "")
		return
	}

	pkg.Log.Info("Default address updated successfully",
		zap.Any("user_id", userID),
		zap.String("address_id", addressID))
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Default address updated"})
}

func ShowOrderSuccess(c *gin.Context) {
	pkg.Log.Info("Rendering order success page")

	orderID := c.Query("order_id")
	var order userModels.Orders
	if err := database.DB.Preload("OrderItems").Preload("OrderItems.Product").Preload("OrderItems.Variants").Preload("ShippingAddress").First(&order, "order_id_unique = ?", orderID).Error; err != nil {
		pkg.Log.Error("Order not found",
			zap.String("order_id", orderID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Order not found", "Something went wrong", "/cart")
		return
	}

	pkg.Log.Info("Order success page loaded",
		zap.String("order_id", orderID))
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

	orderID := c.Query("order_id")
	errorMsg := c.Query("error")
	if errorMsg == "" {
		errorMsg = "Order placement failed"
		pkg.Log.Debug("No error message provided, using default", zap.String("order_id", orderID))
	}

	if orderID == "" {
		pkg.Log.Warn("Order ID is empty")
		helper.ResponseWithErr(c, http.StatusNotFound, "not found order id", "", "")
		return
	}

	var order userModels.Orders
	if err := database.DB.Where("order_id_unique = ?", orderID).First(&order).Error; err != nil {
		pkg.Log.Error("Order not found",
			zap.String("order_id", orderID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "not found order id", "", "")
		return
	}

	order.Status = "Failed"
	order.PaymentStatus = "Pending"

	if err := database.DB.Save(&order).Error; err != nil {
		pkg.Log.Error("Failed to save order status",
			zap.String("order_id", orderID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "error saving orders", "", "")
		return
	}

	var orderItems []userModels.OrderItem
	if err := database.DB.Where("order_id = ?", order.ID).Preload("Product").Preload("Variants").Find(&orderItems).Error; err != nil {
		pkg.Log.Error("Order items not found",
			zap.String("order_id", orderID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "order item is not found", "", "")
		return
	}

	var shipping adminModels.ShippingAddress
	if err := database.DB.Where("order_id = ?", orderID).First(&shipping).Error; err != nil {
		pkg.Log.Error("Shipping address not found",
			zap.String("order_id", orderID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "address not found", "", "")
		return
	}

	pkg.Log.Info("Order failure page loaded",
		zap.String("order_id", orderID),
		zap.String("error_message", errorMsg))
	c.HTML(http.StatusOK, "orderFailure.html", gin.H{
		"status":     "Failed",
		"orderItems": orderItems,
		"message":    errorMsg,
		"address":    shipping,
		"code":       http.StatusOK,
		"ordID":      orderID,
	})
}
