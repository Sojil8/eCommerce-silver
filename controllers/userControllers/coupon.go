package controllers

import (
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
)

type couponApplyReq struct {
	CouponCode string `json:"coupon_code" binding:"required"`
}

type addCouponReq struct {
	CouponCode         string    `json:"couponcode" binding:"required"`
	DiscountPercentage float64   `json:"discount_percentage" binding:"required,gt=0"`
	MinAmount          float64   `json:"min_amount" binding:"required,gte=0"`
	MaxAmount          float64   `json:"max_amount" binding:"required,gte=0"`
	ExpiryDate         time.Time `json:"expirydate" binding:"required"`
	UsageLimit         int       `json:"usage_limit" binding:"required,gt=0"`
	IsActive           bool      `json:"is_active"`
}

func ApplyCoupon(c *gin.Context) {
	userID, _ := c.Get("id")

	var req couponApplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", "Please provide coupon code", "")
		return
	}

	var coupon adminModels.Coupons
	if err := database.DB.Where("coupon_code = ? AND is_active = ? AND expiry_date > ?",
		req.CouponCode, true, time.Now()).
		First(&coupon).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid coupon", "Coupon not found or expired", "")
		return
	}

	if coupon.UsedCount >= coupon.UsageLimit {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Coupon limit reached", "This coupon has reached its usage limit", "")
		return
	}

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).
		Preload("CartItems.Product").
		Preload("CartItems.Variants").
		First(&cart).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Cart not found", "Please add items to cart", "")
		return
	}

	totalPrice := cart.TotalPrice
	if totalPrice < coupon.MinPurchaseAmount || (coupon.MaxPurchaseAmount > 0 && totalPrice > coupon.MaxPurchaseAmount) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid amount",
			"Coupon not applicable for this cart amount", "")
		return
	}

	var existingOrderCoupon userModels.Orders
	if err := database.DB.Where("user_id = ? AND coupon_id = ?", userID, coupon.ID).
		First(&existingOrderCoupon).Error; err == nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Coupon already used",
			"You have already used this coupon", "")
		return
	}

	cart.CouponID = coupon.ID
	if err := database.DB.Save(&cart).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to apply coupon", "Database error", "")
		return
	}

	discount := totalPrice * (coupon.DiscountPercentage / 100)
	finalPrice := totalPrice + shipping - discount

	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"message":     "Coupon applied successfully",
		"discount":    discount,
		"final_price": finalPrice,
		"coupon_id":   coupon.ID,
		"subtotal":    totalPrice,
		"shipping":    shipping,
	})
}

func GetAvailableCoupons(c *gin.Context) {
	userID, _ := c.Get("id")

	var coupons []adminModels.Coupons
	if err := database.DB.Where("is_active = ? AND expiry_date > ?", true, time.Now()).Find(&coupons).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch coupons", "Database error", "")
		return
	}

	var userOrders []userModels.Orders
	if err := database.DB.Where("user_id = ?", userID).Find(&userOrders).Error; err != nil {
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
			"max_purchase_amount": coupon.MaxPurchaseAmount,
			"is_used":             isUsed,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"coupons": responseCoupons,
	})
}

func RemoveCoupon(c *gin.Context) {
	userID, _ := c.Get("id")

	var cart userModels.Cart
	if err := database.DB.Where("user_id = ?", userID).
		Preload("CartItems.Product").
		Preload("CartItems.Variants").
		First(&cart).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Cart not found", "Please add items to cart", "")
		return
	}

	if cart.CouponID == 0 {
		helper.ResponseWithErr(c, http.StatusBadRequest, "No coupon applied", "No coupon to remove", "")
		return
	}

	cart.CouponID = 0
	if err := database.DB.Save(&cart).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to remove coupon", "Database error", "")
		return
	}

	finalPrice := cart.TotalPrice + shipping

	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"message":     "Coupon removed successfully",
		"final_price": finalPrice,
		"subtotal":    cart.TotalPrice,
		"shipping":    shipping,
		"discount":    0.0,
	})
}