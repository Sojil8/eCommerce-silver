package helper

import (
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

func UpdateExpiredCoupons() {
	pkg.Log.Debug("Starting coupon expiry check")

	var coupons []adminModels.Coupons
	if err := database.DB.Where("is_active = ? AND expiry_date < ?", true, time.Now()).Find(&coupons).Error; err != nil {
		pkg.Log.Error("Failed to fetch expired coupons",
			zap.Error(err))
		return
	}

	if len(coupons) == 0 {
		pkg.Log.Info("No expired coupons found")
		return
	}

	for _, coupon := range coupons {
		coupon.IsActive = false
		if err := database.DB.Save(&coupon).Error; err != nil {
			pkg.Log.Error("Failed to deactivate coupon",
				zap.String("couponCode", coupon.CouponCode),
				zap.Error(err))
		} else {
			pkg.Log.Info("Coupon deactivated due to expiry",
				zap.String("couponCode", coupon.CouponCode),
				zap.Time("expiryDate", coupon.ExpiryDate))
		}
	}

	pkg.Log.Info("Coupon expiry check completed",
		zap.Int("couponsProcessed", len(coupons)))
}

func StartCouponExpiryScheduler() {
	pkg.Log.Debug("Initializing coupon expiry scheduler")

	c := cron.New()
	_, err := c.AddFunc("0 0 * * *", UpdateExpiredCoupons)
	if err != nil {
		pkg.Log.Fatal("Failed to schedule coupon expiry task",
			zap.Error(err))
	}

	c.Start()
	pkg.Log.Info("Coupon expiry scheduler started",
		zap.String("schedule", "0 0 * * *"))
}