package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func ShowCoupon(c *gin.Context) {
	pkg.Log.Info("Handling request to show coupons")

	var coupons []adminModels.Coupons
	if err := database.DB.Find(&coupons).Error; err != nil {
		pkg.Log.Error("Failed to fetch coupons", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to list coupons", err.Error(), "")
		return
	}
	pkg.Log.Debug("Coupons retrieved", zap.Int("count", len(coupons)))

	c.HTML(http.StatusOK, "coupon.html", gin.H{
		"status":  "ok",
		"Coupons": coupons,
	})
	pkg.Log.Info("Rendering coupon.html", zap.Int("coupon_count", len(coupons)))
}

type CouponRequest struct {
	CouponCode         string    `json:"couponcode" binding:"required"`
	DiscountPercentage float64   `json:"discount_percentage" binding:"required,gt=0"`
	MinPurchaseAmount  float64   `json:"min_purchase_amount" binding:"gte=0"`
	ExpiryDate         time.Time `json:"expirydate" binding:"required"`
	UsageLimit         int       `json:"usage_limit" binding:"gte=0"`
	IsActive           bool      `json:"is_active"`
}

func AddCoupon(c *gin.Context) {
	pkg.Log.Info("Handling request to add coupon")

	var req CouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Log.Error("Failed to bind JSON data", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}
	pkg.Log.Debug("Received coupon data",
		zap.String("coupon_code", req.CouponCode),
		zap.Float64("discount_percentage", req.DiscountPercentage),
		zap.Float64("min_purchase_amount", req.MinPurchaseAmount),
		zap.Time("expiry_date", req.ExpiryDate),
		zap.Int("usage_limit", req.UsageLimit),
		zap.Bool("is_active", req.IsActive))

	if req.ExpiryDate.Before(time.Now()) {
		pkg.Log.Warn("Invalid expiry date, must be in the future", zap.Time("expiry_date", req.ExpiryDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid expiry date", "Expiry date must be in the future", "")
		return
	}

	if req.DiscountPercentage >= 50 {
		pkg.Log.Warn("Discount percentage too high", zap.Float64("discount_percentage", req.DiscountPercentage))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Discount percentage too high", "Discount percentage must be less than 50", "")
		return
	}

	var existingCoupon adminModels.Coupons
	if err := database.DB.Where("coupon_code = ?", req.CouponCode).First(&existingCoupon).Error; err == nil {
		pkg.Log.Warn("Coupon code already exists", zap.String("coupon_code", req.CouponCode))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Coupon code exists", "Coupon code already exists", "")
		return
	}

	coupon := adminModels.Coupons{
		CouponCode:         req.CouponCode,
		DiscountPercentage: req.DiscountPercentage,
		MinPurchaseAmount:  req.MinPurchaseAmount,
		ExpiryDate:         req.ExpiryDate,
		UsageLimit:         req.UsageLimit,
		IsActive:           req.IsActive,
		UsedCount:          0,
	}

	if err := database.DB.Create(&coupon).Error; err != nil {
		pkg.Log.Error("Failed to create coupon", zap.String("coupon_code", coupon.CouponCode), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot insert data to the database", err.Error(), "")
		return
	}

	pkg.Log.Info("Coupon created successfully", zap.Uint("id", coupon.ID), zap.String("coupon_code", coupon.CouponCode))
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Coupon added successfully",
		"coupon": gin.H{
			"couponcode":          coupon.CouponCode,
			"discount_percentage": coupon.DiscountPercentage,
			"min_purchase_amount": coupon.MinPurchaseAmount,
			"expirydate":          coupon.ExpiryDate,
			"usage_limit":         coupon.UsageLimit,
			"is_active":           coupon.IsActive,
		},
	})
}

func GetCoupon(c *gin.Context) {
	pkg.Log.Info("Handling request to get coupon")

	id := c.Param("id")
	couponID, err := strconv.Atoi(id)
	if err != nil {
		pkg.Log.Warn("Invalid coupon ID", zap.String("id", id), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid coupon ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed coupon ID", zap.Int("id", couponID))

	var coupon adminModels.Coupons
	if err := database.DB.First(&coupon, couponID).Error; err != nil {
		pkg.Log.Error("Coupon not found", zap.Int("id", couponID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Coupon not found", err.Error(), "")
		return
	}
	pkg.Log.Debug("Coupon retrieved", zap.Uint("id", coupon.ID), zap.String("coupon_code", coupon.CouponCode))

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"coupon": gin.H{
			"couponcode":          coupon.CouponCode,
			"discount_percentage": coupon.DiscountPercentage,
			"min_purchase_amount": coupon.MinPurchaseAmount,
			"expirydate":          coupon.ExpiryDate,
			"usage_limit":         coupon.UsageLimit,
			"used_count":          coupon.UsedCount,
			"is_active":           coupon.IsActive,
		},
	})
}

func EditCoupon(c *gin.Context) {
	pkg.Log.Info("Handling request to edit coupon")

	couponIDStr := c.Param("id")
	couponID, err := strconv.Atoi(couponIDStr)
	if err != nil {
		pkg.Log.Warn("Invalid coupon ID", zap.String("id", couponIDStr), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid coupon ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed coupon ID", zap.Int("id", couponID))

	var req CouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Log.Error("Failed to bind JSON data", zap.Int("id", couponID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}
	pkg.Log.Debug("Received coupon update data",
		zap.String("coupon_code", req.CouponCode),
		zap.Float64("discount_percentage", req.DiscountPercentage),
		zap.Float64("min_purchase_amount", req.MinPurchaseAmount),
		zap.Time("expiry_date", req.ExpiryDate),
		zap.Int("usage_limit", req.UsageLimit),
		zap.Bool("is_active", req.IsActive))

	var coupon adminModels.Coupons
	if err := database.DB.First(&coupon, couponID).Error; err != nil {
		pkg.Log.Error("Coupon not found", zap.Int("id", couponID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Coupon not found", err.Error(), "")
		return
	}
	pkg.Log.Debug("Coupon retrieved", zap.Uint("id", coupon.ID), zap.String("coupon_code", coupon.CouponCode))

	if req.CouponCode != coupon.CouponCode {
		var existingCoupon adminModels.Coupons
		if err := database.DB.Where("coupon_code = ? AND id != ?", req.CouponCode, couponID).
			First(&existingCoupon).Error; err == nil {
			pkg.Log.Warn("Coupon code already exists", zap.String("coupon_code", req.CouponCode), zap.Int("id", couponID))
			helper.ResponseWithErr(c, http.StatusBadRequest, "Coupon code exists", "Coupon code already exists", "")
			return
		}
	}

	if req.ExpiryDate.Before(time.Now()) {
		pkg.Log.Warn("Invalid expiry date, must be in the future", zap.Time("expiry_date", req.ExpiryDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid expiry date", "Expiry date must be in the future", "")
		return
	}

	if req.DiscountPercentage >= 50 {
		pkg.Log.Warn("Discount percentage too high", zap.Float64("discount_percentage", req.DiscountPercentage))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Discount percentage too high", "Discount percentage must be less than 50", "")
		return
	}

	coupon.CouponCode = req.CouponCode
	coupon.DiscountPercentage = req.DiscountPercentage
	coupon.MinPurchaseAmount = req.MinPurchaseAmount
	coupon.ExpiryDate = req.ExpiryDate
	coupon.UsageLimit = req.UsageLimit
	coupon.IsActive = req.IsActive

	if err := database.DB.Save(&coupon).Error; err != nil {
		pkg.Log.Error("Failed to update coupon", zap.Uint("id", coupon.ID), zap.String("coupon_code", coupon.CouponCode), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot update the coupon", err.Error(), "")
		return
	}

	pkg.Log.Info("Coupon updated successfully", zap.Uint("id", coupon.ID), zap.String("coupon_code", coupon.CouponCode))
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Coupon updated successfully",
		"coupon": gin.H{
			"couponcode":          coupon.CouponCode,
			"discount_percentage": coupon.DiscountPercentage,
			"min_purchase_amount": coupon.MinPurchaseAmount,
			"expirydate":          coupon.ExpiryDate,
			"usage_limit":         coupon.UsageLimit,
			"is_active":           coupon.IsActive,
		},
	})
}

func DeleteCoupon(c *gin.Context) {
	pkg.Log.Info("Handling request to delete coupon")

	id := c.Param("id")
	couponID, err := strconv.Atoi(id)
	if err != nil {
		pkg.Log.Warn("Invalid coupon ID", zap.String("id", id), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid coupon ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed coupon ID", zap.Int("id", couponID))

	var coupon adminModels.Coupons
	if err := database.DB.First(&coupon, couponID).Error; err != nil {
		pkg.Log.Error("Coupon not found", zap.Int("id", couponID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Coupon not found", err.Error(), "")
		return
	}
	pkg.Log.Debug("Coupon retrieved", zap.Uint("id", coupon.ID), zap.String("coupon_code", coupon.CouponCode))

	if err := database.DB.Delete(&coupon).Error; err != nil {
		pkg.Log.Error("Failed to delete coupon", zap.Uint("id", coupon.ID), zap.String("coupon_code", coupon.CouponCode), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot delete the coupon", err.Error(), "")
		return
	}

	pkg.Log.Info("Coupon deleted successfully", zap.Uint("id", coupon.ID), zap.String("coupon_code", coupon.CouponCode))
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Coupon deleted successfully",
	})
}
