package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type couponApplyReq struct {
	CouponCode string `json:"coupon_code" binding:"required"`
}

func ApplyCoupon(c *gin.Context) {
	pkg.Log.Info("Starting coupon application process")

	userID, _ := c.Get("id")

	var req couponApplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Log.Error("Failed to bind JSON request",
			zap.Any("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", "Please provide coupon code", "")
		return
	}

	pkg.Log.Debug("Coupon application request",
		zap.Any("user_id", userID),
		zap.String("coupon_code", req.CouponCode))

	var coupon adminModels.Coupons
	if err := database.DB.Where("coupon_code = ? AND is_active = ? AND expiry_date > ?",
		req.CouponCode, true, time.Now()).
		First(&coupon).Error; err != nil {
		pkg.Log.Warn("Invalid or expired coupon",
			zap.Any("user_id", userID),
			zap.String("coupon_code", req.CouponCode),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid coupon", "Coupon not found or expired", "")
		return
	}

	if coupon.UsedCount >= coupon.UsageLimit {
		pkg.Log.Warn("Coupon usage limit reached",
			zap.Any("user_id", userID),
			zap.String("coupon_code", coupon.CouponCode),
			zap.Int("used_count", coupon.UsedCount),
			zap.Int("usage_limit", coupon.UsageLimit))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Coupon limit reached", "This coupon has reached its usage limit", "")
		return
	}

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).
		Preload("CartItems.Product").
		Preload("CartItems.Variants").
		First(&cart).Error; err != nil {
		pkg.Log.Warn("Cart not found",
			zap.Any("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Cart not found", "Please add items to cart", "")
		return
	}

	var originalTotalPrice float64
	var totalPrice float64

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
			itemOriginalPrice := offerDetails.OriginalPrice * float64(item.Quantity)
			itemDiscountedPrice := offerDetails.DiscountedPrice * float64(item.Quantity)

			originalTotalPrice += itemOriginalPrice
			totalPrice += itemDiscountedPrice
		} else {
			pkg.Log.Warn("Invalid cart item",
				zap.Any("user_id", userID),
				zap.Uint("cart_id", cart.ID),
				zap.Uint("product_id", item.ProductID),
				zap.Uint("variant_id", item.VariantsID),
				zap.Bool("product_listed", item.Product.IsListed),
				zap.Bool("product_in_stock", isInStock),
				zap.Int("variant_stock", int(variant.Stock)),
				zap.Uint("quantity", item.Quantity))
		}
	}

	cart.OriginalTotalPrice = originalTotalPrice
	cart.TotalPrice = totalPrice
	if err := database.DB.Save(&cart).Error; err != nil {
		pkg.Log.Error("Failed to update cart",
			zap.Any("user_id", userID),
			zap.Uint("cart_id", cart.ID),
			zap.Float64("original_total_price", originalTotalPrice),
			zap.Float64("total_price", totalPrice),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update cart", "Database error", "")
		return
	}

	if originalTotalPrice < coupon.MinPurchaseAmount {
		pkg.Log.Warn("Cart total below minimum purchase amount for coupon",
			zap.Any("user_id", userID),
			zap.String("coupon_code", coupon.CouponCode),
			zap.Float64("cart_total", originalTotalPrice),
			zap.Float64("min_purchase_amount", coupon.MinPurchaseAmount))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid amount",
			fmt.Sprintf("Minimum purchase amount for this coupon is %.2f, your cart total is %.2f",
				coupon.MinPurchaseAmount, originalTotalPrice), "")
		return
	}

	var existingOrderCoupon userModels.Orders
	if err := database.DB.Where("user_id = ? AND coupon_id = ?", userID, coupon.ID).
		First(&existingOrderCoupon).Error; err == nil {
		pkg.Log.Warn("Coupon already used by user",
			zap.Any("user_id", userID),
			zap.String("coupon_code", coupon.CouponCode),
			zap.Uint("coupon_id", coupon.ID))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Coupon already used",
			"You have already used this coupon", "")
		return
	}

	cart.CouponID = coupon.ID
	if err := database.DB.Save(&cart).Error; err != nil {
		pkg.Log.Error("Failed to apply coupon to cart",
			zap.Any("user_id", userID),
			zap.Uint("cart_id", cart.ID),
			zap.String("coupon_code", coupon.CouponCode),
			zap.Uint("coupon_id", coupon.ID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to apply coupon", "Database error", "")
		return
	}

	discount := originalTotalPrice * (coupon.DiscountPercentage / 100)
	shipping := helper.CalculateShipping(cart.TotalPrice)
	finalPrice := totalPrice + shipping - discount

	pkg.Log.Info("Coupon applied successfully",
		zap.Any("user_id", userID),
		zap.String("coupon_code", coupon.CouponCode),
		zap.Uint("coupon_id", coupon.ID),
		zap.Float64("discount", discount),
		zap.Float64("final_price", finalPrice))

	c.JSON(http.StatusOK, gin.H{
		"status":          "ok",
		"message":         "Coupon applied successfully",
		"coupon_discount": discount,
		"final_price":     finalPrice,
		"coupon_id":       coupon.ID,
		"subtotal":        totalPrice,
		"original_total":  originalTotalPrice,
		"shipping":        shipping,
	})
}

func GetAvailableCoupons(c *gin.Context) {
	pkg.Log.Info("Fetching available coupons")

	userID, _ := c.Get("id")

	var coupons []adminModels.Coupons
	if err := database.DB.Where("is_active = ? AND expiry_date > ?", true, time.Now()).Find(&coupons).Error; err != nil {
		pkg.Log.Error("Failed to fetch coupons",
			zap.Any("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch coupons", "Database error", "")
		return
	}

	var userOrders []userModels.Orders
	if err := database.DB.Where("user_id = ?", userID).Find(&userOrders).Error; err != nil {
		pkg.Log.Error("Failed to fetch user orders",
			zap.Any("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch user orders", "Database error", "")
		return
	}

	usedCouponIDs := make(map[uint]bool)
	for _, order := range userOrders {
		if order.CouponID != 0 {
			usedCouponIDs[order.CouponID] = true
		}
	}

	responseCoupons := []map[string]interface{}{}
	for _, coupon := range coupons {
		isUsed := usedCouponIDs[coupon.ID] || coupon.UsedCount >= coupon.UsageLimit
		responseCoupons = append(responseCoupons, map[string]interface{}{
			"coupon_code":         coupon.CouponCode,
			"discount_percentage": coupon.DiscountPercentage,
			"min_purchase_amount": coupon.MinPurchaseAmount,
			"is_used":             isUsed,
		})
	}

	pkg.Log.Info("Available coupons fetched successfully",
		zap.Any("user_id", userID),
		zap.Int("coupon_count", len(responseCoupons)))

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"coupons": responseCoupons,
	})
}

func RemoveCoupon(c *gin.Context) {
	pkg.Log.Info("Starting coupon removal process")

	userID, _ := c.Get("id")

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).
		Preload("CartItems.Product").
		Preload("CartItems.Variants").
		First(&cart).Error; err != nil {
		pkg.Log.Warn("Cart not found",
			zap.Any("user_id", userID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Cart not found", "Please add items to cart", "")
		return
	}

	if cart.CouponID == 0 {
		pkg.Log.Warn("No coupon applied to cart",
			zap.Any("user_id", userID),
			zap.Uint("cart_id", cart.ID))
		helper.ResponseWithErr(c, http.StatusBadRequest, "No coupon applied", "No coupon to remove", "")
		return
	}

	cart.CouponID = 0
	if err := database.DB.Save(&cart).Error; err != nil {
		pkg.Log.Error("Failed to remove coupon from cart",
			zap.Any("user_id", userID),
			zap.Uint("cart_id", cart.ID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to remove coupon", "Database error", "")
		return
	}

	shipping := helper.CalculateShipping(cart.TotalPrice)
	finalPrice := cart.TotalPrice + shipping

	pkg.Log.Info("Coupon removed successfully",
		zap.Any("user_id", userID),
		zap.Uint("cart_id", cart.ID),
		zap.Float64("final_price", finalPrice))

	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"message":     "Coupon removed successfully",
		"final_price": finalPrice,
		"subtotal":    cart.TotalPrice,
		"shipping":    shipping,
		"discount":    0.0,
	})
}
