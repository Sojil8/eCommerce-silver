package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
)

type couponApplyReq struct {
	CouponCode string `json:"coupon_code" binding:"required"`
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
		}
	}

	cart.OriginalTotalPrice = originalTotalPrice
	cart.TotalPrice = totalPrice
	if err := database.DB.Save(&cart).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update cart", "Database error", "")
		return
	}

	if originalTotalPrice < coupon.MinPurchaseAmount {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid amount",
			fmt.Sprintf("Minimum purchase amount for this coupon is %.2f, your cart total is %.2f",
				coupon.MinPurchaseAmount, originalTotalPrice), "")
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

	discount := originalTotalPrice * (coupon.DiscountPercentage / 100)
	shipping := helper.CalculateShipping(cart.TotalPrice)
	finalPrice := totalPrice + shipping - discount

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

	shipping := helper.CalculateShipping(cart.TotalPrice)

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
