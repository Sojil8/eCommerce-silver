package controllers

import (
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
)

type SalesReport struct {
	TotalOrders   int64          `json:"total_orders"`
	TotalAmount   float64        `json:"total_amount"`
	TotalDiscount float64        `json:"total_discount"`
	TotalCoupons  int            `json:"total_coupons"`
	Orders        []OrderDetails `json:"orders"`
}

type OrderDetails struct {
	OrderID       string    `json:"order_id"`
	UserName      string    `json:"user_name"`
	OrderDate     time.Time `json:"order_date"`
	TotalPrice    float64   `json:"total_price"`
	Discount      float64   `json:"discount"`
	CouponCode    string    `json:"coupon_code"`
	PaymentMethod string    `json:"payment_method"`
	Status        string    `json:"status"`
}

type DateFilter struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
}

func GetSalesReport(c *gin.Context) {
	db := database.GetDB()
	var filter DateFilter
	var startDate, endDate time.Time
	var err error

	reportType := c.Query("type")

	if reportType == "custom" {
		if err := c.ShouldBindQuery(&filter); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
			return
		}

		startDate, err = time.Parse("2006-01-02", filter.StartDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date format"})
			return
		}

		endDate, err = time.Parse("2006-01-02", filter.EndDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date format"})
			return
		}
		endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	} else {
		endDate = time.Now()
		switch reportType {
		case "daily":
			startDate = endDate.Truncate(24 * time.Hour)
		case "weekly":
			startDate = endDate.AddDate(0, 0, -7)
		case "monthly":
			startDate = endDate.AddDate(0, -1, 0)
		case "yearly":
			startDate = endDate.AddDate(-1, 0, 0)
		default:
			startDate = endDate.AddDate(0, 0, -30) // Default to last 30 days
		}
	}

	var orders []userModels.Orders
	var report SalesReport

	// Query orders within date range
	if err := db.Preload("User").Preload("OrderItems.Product").
		Where("order_date BETWEEN ? AND ?", startDate, endDate).
		Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	report.TotalOrders = int64(len(orders))
	report.TotalDiscount = 0
	report.TotalAmount = 0
	report.TotalCoupons = 0

	for _, order := range orders {
		var coupon adminModels.Coupons
		couponCode := ""
		if order.CouponID != 0 {
			if err := db.First(&coupon, order.CouponID).Error; err == nil {
				couponCode = coupon.CouponCode
				report.TotalCoupons++
			}
		}

		report.TotalAmount += order.TotalPrice
		report.TotalDiscount += order.Discount

		orderDetail := OrderDetails{
			OrderID:       order.OrderIdUnique,
			UserName:      order.User.UserName,
			OrderDate:     order.OrderDate,
			TotalPrice:    order.TotalPrice,
			Discount:      order.Discount,
			CouponCode:    couponCode,
			PaymentMethod: order.PaymentMethod,
			Status:        order.Status,
		}
		report.Orders = append(report.Orders, orderDetail)
	}

	c.HTML(http.StatusOK,"salesReport.html",gin.H{
		"status":"ok",
	})
}
