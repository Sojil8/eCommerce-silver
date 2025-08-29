package controllers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DashboardData struct {
	TotalUsers       int64                   `json:"total_users"`
	TotalOrders      int64                   `json:"total_orders"`
	TotalRevenue     float64                 `json:"total_revenue"`
	TotalProducts    int64                   `json:"total_products"`
	ActiveCoupons    int64                   `json:"active_coupons"`
	PendingOrders    int64                   `json:"pending_orders"`
	CompletedOrders  int64                   `json:"completed_orders"`
	CancelledOrders  int64                   `json:"cancelled_orders"`
	TopProducts      []TopProduct            `json:"top_products"`
	RecentOrders     []userModels.Orders     `json:"recent_orders"`
	SalesData        []SalesDataPoint        `json:"sales_data"`
	UserActivityData []UserActivityDataPoint `json:"user_activity_data"`
	InventoryStatus  []InventoryStatusItem   `json:"inventory_status"`
	CouponUsage      []CouponUsageItem       `json:"coupon_usage"`
	MonthlyRevenue   []MonthlyRevenuePoint   `json:"monthly_revenue"`
}

type TopProduct struct {
	ProductName  string  `json:"product_name"`
	TotalSold    int64   `json:"total_sold"`
	Revenue      float64 `json:"revenue"`
	CategoryName string  `json:"category_name"`
}

type SalesDataPoint struct {
	Date   string  `json:"date"`
	Sales  float64 `json:"sales"`
	Orders int64   `json:"orders"`
}

type UserActivityDataPoint struct {
	Date        string `json:"date"`
	NewUsers    int64  `json:"new_users"`
	ActiveUsers int64  `json:"active_users"`
}

type InventoryStatusItem struct {
	ProductName string `json:"product_name"`
	Stock       int64  `json:"stock"`
	Status      string `json:"status"`
}

type CouponUsageItem struct {
	CouponCode    string  `json:"coupon_code"`
	UsedCount     int     `json:"used_count"`
	TotalDiscount float64 `json:"total_discount"`
}

type MonthlyRevenuePoint struct {
	Month   string  `json:"month"`
	Revenue float64 `json:"revenue"`
}

func ShowDashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title": "Admin Dashboard",
	})
}

func GetDashboardData(c *gin.Context) {
	filter := c.DefaultQuery("filter", "weekly")

	var dashboardData DashboardData
	db := database.GetDB()

	// Get total counts
	db.Model(&userModels.Users{}).Count(&dashboardData.TotalUsers)
	db.Model(&userModels.Orders{}).Count(&dashboardData.TotalOrders)
	db.Model(&adminModels.Product{}).Count(&dashboardData.TotalProducts)
	db.Model(&adminModels.Coupons{}).Where("is_active = ?", true).Count(&dashboardData.ActiveCoupons)

	// Get order status counts
	db.Model(&userModels.Orders{}).Where("status IN ?", []string{"Pending", "Confirmed", "Shipped", "Out for Delivery"}).Count(&dashboardData.PendingOrders)
	db.Model(&userModels.Orders{}).Where("status = ?", "Delivered").Count(&dashboardData.CompletedOrders)
	db.Model(&userModels.Orders{}).Where("status IN ?", []string{"Return Rejected", "Cancelled", "Returned"}).Count(&dashboardData.CancelledOrders)

	// Calculate total revenue
	var revenue struct {
		Total float64
	}
	db.Model(&userModels.Orders{}).
		Where("payment_status =(?)", []string{"Paid"}).
		Select("COALESCE(SUM(total_price), 0.0) as total").
		Scan(&revenue)
	dashboardData.TotalRevenue = revenue.Total

	// Get top selling products
	getTopProducts(db, &dashboardData)

	// Get recent orders
	getRecentOrders(db, &dashboardData)

	// Get sales data based on filter
	getSalesData(db, &dashboardData, filter)

	// Get user activity data
	getUserActivityData(db, &dashboardData, filter)

	// Get inventory status
	getInventoryStatus(db, &dashboardData)

	// Get coupon usage
	getCouponUsage(db, &dashboardData)

	// Get monthly revenue
	getMonthlyRevenue(db, &dashboardData)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   dashboardData,
	})
}

func getTopProducts(db *gorm.DB, data *DashboardData) {
	var topProducts []TopProduct

	query := `
		SELECT 
			p.product_name,
			COALESCE(SUM(oi.quantity), 0) as total_sold,
			COALESCE(SUM(oi.item_total), 0) as revenue,
			p.category_name
		FROM products p
		LEFT JOIN order_items oi ON p.id = oi.product_id
		LEFT JOIN orders o ON oi.order_id = o.id
		WHERE o.status NOT IN ('cancelled', 'refunded') OR o.status IS NULL
		GROUP BY p.id, p.product_name, p.category_name
		ORDER BY total_sold DESC
		LIMIT 10
	`

	db.Raw(query).Scan(&topProducts)
	data.TopProducts = topProducts
}

func getRecentOrders(db *gorm.DB, data *DashboardData) {
	var recentOrders []userModels.Orders

	db.Preload("User").
		Preload("OrderItems").
		Preload("OrderItems.Product").
		Order("created_at DESC").
		Limit(10).
		Find(&recentOrders)

	data.RecentOrders = recentOrders
}

func getSalesData(db *gorm.DB, data *DashboardData, filter string) {
	var salesData []SalesDataPoint
	var days int
	var dateFormat string

	switch filter {
	case "daily":
		days = 7
		dateFormat = "2006-01-02"
	case "weekly":
		days = 28
		dateFormat = "2006-01-02"
	case "monthly":
		days = 365
		dateFormat = "2006-01"
	default:
		days = 28
		dateFormat = "2006-01-02"
	}

	query := `
		SELECT 
			DATE(created_at) as date,
			COALESCE(SUM(total_price), 0) as sales,
			COUNT(*) as orders
		FROM orders 
		WHERE created_at >= ? AND status NOT IN ('cancelled', 'refunded')
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	fmt.Println(dateFormat)
	startDate := time.Now().AddDate(0, 0, -days)
	db.Raw(query, startDate).Scan(&salesData)

	data.SalesData = salesData
}

func getUserActivityData(db *gorm.DB, data *DashboardData, filter string) {
	var userActivity []UserActivityDataPoint
	var days int

	switch filter {
	case "daily":
		days = 7
	case "weekly":
		days = 28
	case "monthly":
		days = 365
	default:
		days = 28
	}

	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as new_users,
			COUNT(*) as active_users
		FROM users 
		WHERE created_at >= ?
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`

	startDate := time.Now().AddDate(0, 0, -days)
	db.Raw(query, startDate).Scan(&userActivity)

	data.UserActivityData = userActivity
}

func getInventoryStatus(db *gorm.DB, data *DashboardData) {
	var inventory []InventoryStatusItem

	query := `
		SELECT 
   			 p.product_name,
   			 COALESCE(SUM(v.stock), 0) as stock,
  		  CASE 
       		 WHEN COALESCE(SUM(v.stock), 0) = 0 THEN 'Out of Stock'
      		  WHEN COALESCE(SUM(v.stock), 0) < 10 THEN 'Low Stock'
      	  ELSE 'In Stock'
    		END as status
		FROM products p
			LEFT JOIN variants v ON p.id = v.product_id
		WHERE p.is_listed = true AND v.deleted_at IS NULL
		GROUP BY p.id, p.product_name
		ORDER BY stock ASC
		LIMIT 20
	`

	db.Raw(query).Scan(&inventory)
	data.InventoryStatus = inventory
}

func getCouponUsage(db *gorm.DB, data *DashboardData) {
	var couponUsage []CouponUsageItem

	query := `
		SELECT 
			c.coupon_code,
			c.used_count,
			COALESCE(SUM(o.coupon_discount), 0) as total_discount
		FROM coupons c
		LEFT JOIN orders o ON c.coupon_code = o.coupon_code
		WHERE c.is_active = true
		GROUP BY c.id, c.coupon_code, c.used_count
		ORDER BY c.used_count DESC
		LIMIT 10
	`

	db.Raw(query).Scan(&couponUsage)
	data.CouponUsage = couponUsage
}

func getMonthlyRevenue(db *gorm.DB, data *DashboardData) {
	var monthlyRevenue []MonthlyRevenuePoint

	query := `
		SELECT 
			TO_CHAR(created_at, 'YYYY-MM') as month,
			COALESCE(SUM(total_price), 0) as revenue
		FROM orders 
		WHERE created_at >= ? AND payment_status = 'Paid'
		GROUP BY TO_CHAR(created_at, 'YYYY-MM')
		ORDER BY month ASC
	`

	startDate := time.Now().AddDate(-1, 0, 0)
	db.Raw(query, startDate).Scan(&monthlyRevenue)

	data.MonthlyRevenue = monthlyRevenue
}

func ExportSalesReport(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")
	filter := c.DefaultQuery("filter", "monthly")

	db := database.GetDB()

	var salesData []struct {
		Date         string  `json:"date"`
		Orders       int64   `json:"orders"`
		Revenue      float64 `json:"revenue"`
		ProductsSold int64   `json:"products_sold"`
	}

	var days int
	switch filter {
	case "daily":
		days = 30
	case "weekly":
		days = 84
	case "monthly":
		days = 365
	default:
		days = 365
	}

	query := `
		SELECT 
			DATE(o.created_at) as date,
			COUNT(DISTINCT o.id) as orders,
			COALESCE(SUM(o.total_price), 0) as revenue,
			COALESCE(SUM(oi.quantity), 0) as products_sold
		FROM orders o
		LEFT JOIN order_items oi ON o.id = oi.order_id
		WHERE o.created_at >= ? AND o.status NOT IN ('cancelled', 'refunded')
		GROUP BY DATE(o.created_at)
		ORDER BY date ASC
	`

	startDate := time.Now().AddDate(0, 0, -days)
	db.Raw(query, startDate).Scan(&salesData)

	if format == "csv" {
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=sales_report.csv")

		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// Write header
		writer.Write([]string{"Date", "Orders", "Revenue", "Products Sold"})

		// Write data
		for _, record := range salesData {
			writer.Write([]string{
				record.Date,
				strconv.FormatInt(record.Orders, 10),
				fmt.Sprintf("%.2f", record.Revenue),
				strconv.FormatInt(record.ProductsSold, 10),
			})
		}
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   salesData,
		})
	}
}

func LogAdminAction(c *gin.Context) {
	var actionLog struct {
		Action      string `json:"action" binding:"required"`
		Description string `json:"description"`
		EntityType  string `json:"entity_type"`
		EntityID    uint   `json:"entity_id"`
	}

	if err := c.ShouldBindJSON(&actionLog); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "binding data", "Failed to bind data", "")
		return
	}

	// Here you would typically save to an admin_logs table
	// For now, we'll just return success
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Action logged successfully",
	})
}
