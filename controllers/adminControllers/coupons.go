package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
)

func ShowCoupon(c *gin.Context) {
	var coupons []adminModels.Coupons
	if err := database.DB.Find(&coupons).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to list coupons", "", "")
		return
	}
	c.HTML(http.StatusOK, "coupon.html", gin.H{
		"status":  "ok",
		"Coupons": coupons,
	})
}

type CouponRequest struct {
	CouponCode         string    `json:"couponcode" binding:"required"`
	DiscountPercentage float64   `json:"discount_percentage" binding:"required,gt=0"`
	MinPurchaseAmount  float64   `json:"min_purchase_amount" binding:"gte=0"`
	MaxPurchaseAmount  float64   `json:"max_purchase_amount" binding:"gte=0"`
	ExpiryDate         time.Time `json:"expirydate" binding:"required"`
	UsageLimit         int       `json:"usage_limit" binding:"gte=0"`
	IsActive           bool      `json:"is_active"`
}

func AddCoupon(c *gin.Context) {
	var req CouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	if req.ExpiryDate.Before(time.Now()) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid expiry date", "Expiry date must be in the future", "")
		return
	}

	if req.DiscountPercentage >=50{
		helper.ResponseWithErr(c,http.StatusBadRequest,"discount percentage should be less than 50","discount percentage should be less than 50","")
		return
	}

	if req.MinPurchaseAmount > req.MaxPurchaseAmount && req.MaxPurchaseAmount > 0 {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid amounts", "Min purchase amount cannot exceed max purchase amount", "")
		return
	}

	var existingCoupon adminModels.Coupons
	if err := database.DB.Where("coupon_code = ?", req.CouponCode).First(&existingCoupon).Error; err == nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Coupon code exists", "Coupon code already exists", "")
		return
	}
	

	coupon := adminModels.Coupons{
		CouponCode:         req.CouponCode,
		DiscountPercentage: req.DiscountPercentage,
		MinPurchaseAmount:  req.MinPurchaseAmount,
		MaxPurchaseAmount:  req.MaxPurchaseAmount,
		ExpiryDate:         req.ExpiryDate,
		UsageLimit:         req.UsageLimit,
		IsActive:           req.IsActive,
		UsedCount:          0,
	}

	if err := database.DB.Create(&coupon).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot insert data to the database", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Coupon added successfully",
		"coupon": gin.H{
			"couponcode":         coupon.CouponCode,
			"discount_percentage": coupon.DiscountPercentage,
			"min_purchase_amount": coupon.MinPurchaseAmount,
			"max_purchase_amount": coupon.MaxPurchaseAmount,
			"expirydate":         coupon.ExpiryDate,
			"usage_limit":        coupon.UsageLimit,
			"is_active":          coupon.IsActive,
		},
	})
}

func GetCoupon(c *gin.Context) {
	id := c.Param("id")
	couponID, err := strconv.Atoi(id)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid coupon ID", err.Error(), "")
		return
	}

	var coupon adminModels.Coupons
	if err := database.DB.First(&coupon, couponID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Coupon not found", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"coupon": gin.H{
			"couponcode":         coupon.CouponCode,
			"discount_percentage": coupon.DiscountPercentage,
			"min_purchase_amount": coupon.MinPurchaseAmount,
			"max_purchase_amount": coupon.MaxPurchaseAmount,
			"expirydate":         coupon.ExpiryDate,
			"usage_limit":        coupon.UsageLimit,
			"used_count":         coupon.UsedCount,
			"is_active":          coupon.IsActive,
		},
	})
}

func EditCoupon(c *gin.Context) {
	couponIDStr := c.Param("id")
	couponID, err := strconv.Atoi(couponIDStr)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Failed to convert ID", err.Error(), "")
		return
	}

	var req CouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	var coupon adminModels.Coupons
	if err := database.DB.First(&coupon, couponID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Coupon not found", err.Error(), "")
		return
	}

	if req.CouponCode != coupon.CouponCode {
		var existingCoupon adminModels.Coupons
		if err := database.DB.Where("coupon_code = ? AND id != ?", req.CouponCode, couponID).
			First(&existingCoupon).Error; err == nil {
			helper.ResponseWithErr(c, http.StatusBadRequest, "Coupon code exists", "Coupon code already exists", "")
			return
		}
	}

	if req.ExpiryDate.Before(time.Now()) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid expiry date", "Expiry date must be in the future", "")
		return
	}

	if req.MinPurchaseAmount > req.MaxPurchaseAmount && req.MaxPurchaseAmount > 0 {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid amounts", "Min purchase amount cannot exceed max purchase amount", "")
		return
	}

	if req.DiscountPercentage >= 50{
		helper.ResponseWithErr(c,http.StatusBadRequest,"discount percentage should be less than 50","discount percentage should be less than 50","")
		return
	}

	coupon.CouponCode = req.CouponCode
	coupon.DiscountPercentage = req.DiscountPercentage
	coupon.MinPurchaseAmount = req.MinPurchaseAmount
	coupon.MaxPurchaseAmount = req.MaxPurchaseAmount
	coupon.ExpiryDate = req.ExpiryDate
	coupon.UsageLimit = req.UsageLimit
	coupon.IsActive = req.IsActive

	if err := database.DB.Save(&coupon).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot update the coupon", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Coupon updated successfully",
		"coupon": gin.H{
			"couponcode":         coupon.CouponCode,
			"discount_percentage": coupon.DiscountPercentage,
			"min_purchase_amount": coupon.MinPurchaseAmount,
			"max_purchase_amount": coupon.MaxPurchaseAmount,
			"expirydate":         coupon.ExpiryDate,
			"usage_limit":        coupon.UsageLimit,
			"is_active":          coupon.IsActive,
		},
	})
}

func DeleteCoupon(c *gin.Context) {
	id := c.Param("id")
	couponID, err := strconv.Atoi(id)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid coupon ID", err.Error(), "")
		return
	}

	var coupon adminModels.Coupons
	if err := database.DB.First(&coupon, couponID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Coupon not found", err.Error(), "")
		return
	}

	if err := database.DB.Delete(&coupon).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot delete the coupon", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Coupon deleted successfully",
	})
}