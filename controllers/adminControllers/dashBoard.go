package controllers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/props"
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
	AvgOrderValue    float64                 `json:"avg_order_value"`
	TopProducts      []TopProduct            `json:"top_products"`
	TopCategories    []TopCategory           `json:"top_categories"`
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

type TopCategory struct {
	CategoryName string  `json:"category_name"`
	TotalSold    int64   `json:"total_sold"`
	Revenue      float64 `json:"revenue"`
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

type SalesReportData struct {
	Date           string  `json:"date"`
	Orders         int64   `json:"orders"`
	Revenue        float64 `json:"revenue"`
	ProductsSold   int64   `json:"products_sold"`
	TotalDiscount  float64 `json:"total_discount"`
	CouponDiscount float64 `json:"coupon_discount"`
	OfferDiscount  float64 `json:"offer_discount"`
	CouponCodes    string  `json:"coupon_codes"`
}

func ShowDashboard(c *gin.Context) {
	filter := c.DefaultQuery("filter", "weekly")
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	var dashboardData DashboardData
	db := database.GetDB()

	var startDate, endDate time.Time
	var useCustomRange bool
	if startDateStr != "" && endDateStr != "" {
		var err1, err2 error
		startDate, err1 = time.Parse("2006-01-02", startDateStr)
		endDate, err2 = time.Parse("2006-01-02", endDateStr)
		if err1 == nil && err2 == nil {
			useCustomRange = true
			endDate = endDate.AddDate(0, 0, 1)
		}
	} else {
		// New: Compute default range based on filter
		useCustomRange = true // Now always true for filters
		now := time.Now()
		endDate = now.AddDate(0, 0, 1) // Tomorrow to include today
		switch filter {
		case "daily":
			startDate = now.AddDate(0, 0, -7) // Last 7 days
		case "weekly":
			startDate = now.AddDate(0, 0, -28) // Last 4 weeks
		case "monthly":
			startDate = now.AddDate(-12, 0, 0) // Last 12 months
		case "yearly":
			startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()) // Jan 1 of current year
		default:
			useCustomRange = false // Fallback to all-time if invalid filter
			// Or set a default like startDate = now.AddDate(0, 0, -90) for 90 days
		}
	}

	query := db.Model(&userModels.Users{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.TotalUsers)

	query = db.Model(&userModels.Orders{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.TotalOrders)

	query = db.Model(&adminModels.Product{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.TotalProducts)

	query = db.Model(&adminModels.Coupons{}).Where("is_active = ?", true)
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.ActiveCoupons)

	query = db.Model(&userModels.Orders{}).Where("status IN ?", []string{"Pending", "Confirmed", "Shipped", "Out for Delivery"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.PendingOrders)

	query = db.Model(&userModels.Orders{}).Where("status = ?", "Delivered")
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.CompletedOrders)

	query = db.Model(&userModels.Orders{}).Where("status IN ?", []string{"Return Rejected", "Cancelled", "Returned"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.CancelledOrders)

	var revenue struct {
		Total float64
	}
	query = db.Model(&userModels.Orders{}).Where("payment_status = ?", "Paid")
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Select("COALESCE(SUM(total_price), 0.0) as total").Scan(&revenue)
	dashboardData.TotalRevenue = revenue.Total

	if dashboardData.TotalOrders > 0 {
		dashboardData.AvgOrderValue = dashboardData.TotalRevenue / float64(dashboardData.TotalOrders)
	} else {
		dashboardData.AvgOrderValue = 0
	}

	getTopProducts(db, &dashboardData, useCustomRange, startDate, endDate)
	getTopCategories(db, &dashboardData, useCustomRange, startDate, endDate)
	getRecentOrders(db, &dashboardData, useCustomRange, startDate, endDate)
	getSalesData(db, &dashboardData, filter, useCustomRange, startDate, endDate)
	getUserActivityData(db, &dashboardData, filter, useCustomRange, startDate, endDate)
	getInventoryStatus(db, &dashboardData, useCustomRange, startDate, endDate)
	getCouponUsage(db, &dashboardData, useCustomRange, startDate, endDate)
	getMonthlyRevenue(db, &dashboardData, useCustomRange, startDate, endDate)

	topProductsJSON, _ := json.Marshal(dashboardData.TopProducts)
	topCategoriesJSON, _ := json.Marshal(dashboardData.TopCategories)
	recentOrdersJSON, _ := json.Marshal(dashboardData.RecentOrders)
	salesDataJSON, _ := json.Marshal(dashboardData.SalesData)
	userActivityDataJSON, _ := json.Marshal(dashboardData.UserActivityData)
	inventoryStatusJSON, _ := json.Marshal(dashboardData.InventoryStatus)
	couponUsageJSON, _ := json.Marshal(dashboardData.CouponUsage)
	monthlyRevenueJSON, _ := json.Marshal(dashboardData.MonthlyRevenue)

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":                "Admin Dashboard",
		"TotalUsers":           dashboardData.TotalUsers,
		"TotalOrders":          dashboardData.TotalOrders,
		"TotalRevenue":         dashboardData.TotalRevenue,
		"TotalProducts":        dashboardData.TotalProducts,
		"ActiveCoupons":        dashboardData.ActiveCoupons,
		"PendingOrders":        dashboardData.PendingOrders,
		"CompletedOrders":      dashboardData.CompletedOrders,
		"CancelledOrders":      dashboardData.CancelledOrders,
		"AvgOrderValue":        dashboardData.AvgOrderValue,
		"TopProductsJSON":      string(topProductsJSON),
		"TopCategoriesJSON":    string(topCategoriesJSON),
		"RecentOrdersJSON":     string(recentOrdersJSON),
		"SalesDataJSON":        string(salesDataJSON),
		"UserActivityDataJSON": string(userActivityDataJSON),
		"InventoryStatusJSON":  string(inventoryStatusJSON),
		"CouponUsageJSON":      string(couponUsageJSON),
		"MonthlyRevenueJSON":   string(monthlyRevenueJSON),
		"Filter":               filter,
		"StartDate":            startDateStr,
		"EndDate":              endDateStr,
	})
}

func GetDashboardData(c *gin.Context) {
	filter := c.DefaultQuery("filter", "weekly")
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	var dashboardData DashboardData
	db := database.GetDB()

	var startDate, endDate time.Time
	var useCustomRange bool
	if startDateStr != "" && endDateStr != "" {
		var err1, err2 error
		startDate, err1 = time.Parse("2006-01-02", startDateStr)
		endDate, err2 = time.Parse("2006-01-02", endDateStr)
		if err1 == nil && err2 == nil {
			useCustomRange = true
			endDate = endDate.AddDate(0, 0, 1)
		}
	}

	query := db.Model(&userModels.Users{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.TotalUsers)

	query = db.Model(&userModels.Orders{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.TotalOrders)

	query = db.Model(&adminModels.Product{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.TotalProducts)

	query = db.Model(&adminModels.Coupons{}).Where("is_active = ?", true)
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.ActiveCoupons)

	query = db.Model(&userModels.Orders{}).Where("status IN ?", []string{"Pending", "Confirmed", "Shipped", "Out for Delivery"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.PendingOrders)

	query = db.Model(&userModels.Orders{}).Where("status = ?", "Delivered")
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.CompletedOrders)

	query = db.Model(&userModels.Orders{}).Where("status IN ?", []string{"Return Rejected", "Cancelled", "Returned"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Count(&dashboardData.CancelledOrders)

	var revenue struct {
		Total float64
	}
	query = db.Model(&userModels.Orders{}).Where("payment_status = ?", "Paid")
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Select("COALESCE(SUM(total_price), 0.0) as total").Scan(&revenue)
	dashboardData.TotalRevenue = revenue.Total

	if dashboardData.TotalOrders > 0 {
		dashboardData.AvgOrderValue = dashboardData.TotalRevenue / float64(dashboardData.TotalOrders)
	} else {
		dashboardData.AvgOrderValue = 0
	}

	getTopProducts(db, &dashboardData, useCustomRange, startDate, endDate)
	getTopCategories(db, &dashboardData, useCustomRange, startDate, endDate)
	getRecentOrders(db, &dashboardData, useCustomRange, startDate, endDate)
	getSalesData(db, &dashboardData, filter, useCustomRange, startDate, endDate)
	getUserActivityData(db, &dashboardData, filter, useCustomRange, startDate, endDate)
	getInventoryStatus(db, &dashboardData, useCustomRange, startDate, endDate)
	getCouponUsage(db, &dashboardData, useCustomRange, startDate, endDate)
	getMonthlyRevenue(db, &dashboardData, useCustomRange, startDate, endDate)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   dashboardData,
	})
}

func getTopProducts(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
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
		WHERE (o.status NOT IN ('cancelled', 'refunded') OR o.status IS NULL)
		%s
		GROUP BY p.id, p.product_name, p.category_name
		ORDER BY total_sold DESC
		LIMIT 10
	`
	var whereClause string
	if useCustomRange {
		whereClause = "AND o.created_at >= ? AND o.created_at < ?"
		query = fmt.Sprintf(query, whereClause)
		db.Raw(query, startDate, endDate).Scan(&topProducts)
	} else {
		query = fmt.Sprintf(query, "")
		db.Raw(query).Scan(&topProducts)
	}
	data.TopProducts = topProducts
}

func getTopCategories(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	var topCategories []TopCategory
	query := `
		SELECT 
			p.category_name,
			COALESCE(SUM(oi.quantity), 0) as total_sold,
			COALESCE(SUM(oi.item_total), 0) as revenue
		FROM products p
		LEFT JOIN order_items oi ON p.id = oi.product_id
		LEFT JOIN orders o ON oi.order_id = o.id
		WHERE (o.status NOT IN ('cancelled', 'refunded') OR o.status IS NULL)
		%s
		GROUP BY p.category_name
		ORDER BY total_sold DESC
		LIMIT 10
	`
	var whereClause string
	if useCustomRange {
		whereClause = "AND o.created_at >= ? AND o.created_at < ?"
		query = fmt.Sprintf(query, whereClause)
		db.Raw(query, startDate, endDate).Scan(&topCategories)
	} else {
		query = fmt.Sprintf(query, "")
		db.Raw(query).Scan(&topCategories)
	}
	data.TopCategories = topCategories
}

func getRecentOrders(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	var recentOrders []userModels.Orders
	query := db.Preload("User").Preload("OrderItems").Preload("OrderItems.Product").Order("created_at DESC").Limit(10)
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Find(&recentOrders)
	data.RecentOrders = recentOrders
}

func getSalesData(db *gorm.DB, data *DashboardData, filter string, useCustomRange bool, startDate, endDate time.Time) {
	var salesData []SalesDataPoint
	var groupBy string

	switch filter {
	case "daily":
		groupBy = "DATE(created_at)"
	case "weekly":
		groupBy = "DATE_TRUNC('week', created_at)"
	case "monthly":
		groupBy = "DATE_TRUNC('month', created_at)"
	case "yearly":
		groupBy = "DATE_TRUNC('year', created_at)"
	default:
		groupBy = "DATE_TRUNC('week', created_at)"
	}

	query := `
		SELECT 
			%s as date_group,
			COALESCE(SUM(total_price), 0) as sales,
			COUNT(*) as orders
		FROM orders 
		WHERE status NOT IN ('cancelled', 'refunded')
		%s
		GROUP BY %s
		ORDER BY date_group ASC
	`
	var whereClause string
	if useCustomRange {
		whereClause = "AND created_at >= ? AND created_at < ?"
		query = fmt.Sprintf(query, groupBy, whereClause, groupBy)
		db.Raw(query, startDate, endDate).Scan(&salesData)
	} else {
		whereClause = "AND created_at >= NOW() - INTERVAL '90 days'"
		query = fmt.Sprintf(query, groupBy, whereClause, groupBy)
		db.Raw(query).Scan(&salesData)
	}

	for i := range salesData {
		if t, err := time.Parse(time.RFC3339, salesData[i].Date); err == nil {
			switch filter {
			case "daily":
				salesData[i].Date = t.Format("Jan 02")
			case "weekly":
				salesData[i].Date = t.Format("Week of Jan 02")
			case "monthly":
				salesData[i].Date = t.Format("Jan 2006")
			case "yearly":
				salesData[i].Date = t.Format("2006")
			}
		}
	}
	data.SalesData = salesData
}

func getUserActivityData(db *gorm.DB, data *DashboardData, filter string, useCustomRange bool, startDate, endDate time.Time) {
	var userActivity []UserActivityDataPoint
	var interval string
	var groupBy string

	switch filter {
	case "daily":
		interval = "7 DAY"
		groupBy = "DATE(created_at)"
	case "weekly":
		interval = "4 WEEK"
		groupBy = "DATE_TRUNC('week', created_at)"
	case "monthly":
		interval = "12 MONTH"
		groupBy = "DATE_TRUNC('month', created_at)"
	case "yearly":
		interval = "1 YEAR"
		groupBy = "DATE_TRUNC('year', created_at)"
	default:
		interval = "4 WEEK"
		groupBy = "DATE_TRUNC('week', created_at)"
	}

	query := `
		SELECT 
			%s as date,
			COUNT(*) as new_users,
			(SELECT COUNT(DISTINCT user_id) FROM user_sessions 
			 WHERE %s = dates.date) as active_users
		FROM (
			SELECT %s as date
			FROM generate_series(
				%s,
				%s,
				INTERVAL '1 day'
			) dates
		) dates
		LEFT JOIN users u ON %s = dates.date
		%s
		GROUP BY dates.date
		ORDER BY dates.date ASC
	`
	var whereClause, startExpr, endExpr string
	if useCustomRange {
		startExpr = "?"
		endExpr = "?"
		whereClause = "AND u.created_at >= ? AND u.created_at < ?"
		query = fmt.Sprintf(query, groupBy, groupBy, groupBy, startExpr, endExpr, groupBy, whereClause)
		db.Raw(query, startDate, endDate, startDate, endDate).Scan(&userActivity)
	} else {
		startExpr = "NOW() - INTERVAL '" + interval + "'"
		endExpr = "NOW()"
		query = fmt.Sprintf(query, groupBy, groupBy, groupBy, startExpr, endExpr, groupBy, "")
		db.Raw(query).Scan(&userActivity)
	}
	data.UserActivityData = userActivity
}

func getInventoryStatus(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
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
		%s
		GROUP BY p.id, p.product_name
		ORDER BY stock ASC
		LIMIT 20
	`
	var whereClause string
	if useCustomRange {
		whereClause = "AND p.created_at >= ? AND p.created_at < ?"
		query = fmt.Sprintf(query, whereClause)
		db.Raw(query, startDate, endDate).Scan(&inventory)
	} else {
		query = fmt.Sprintf(query, "")
		db.Raw(query).Scan(&inventory)
	}
	data.InventoryStatus = inventory
}

func getCouponUsage(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	var couponUsage []CouponUsageItem
	query := `
		SELECT 
			c.coupon_code,
			c.used_count,
			COALESCE(SUM(o.coupon_discount), 0) as total_discount
		FROM coupons c
		LEFT JOIN orders o ON c.coupon_code = o.coupon_code
		WHERE c.is_active = true
		%s
		GROUP BY c.id, c.coupon_code, c.used_count
		ORDER BY c.used_count DESC
		LIMIT 10
	`
	var whereClause string
	if useCustomRange {
		whereClause = "AND o.created_at >= ? AND o.created_at < ?"
		query = fmt.Sprintf(query, whereClause)
		db.Raw(query, startDate, endDate).Scan(&couponUsage)
	} else {
		query = fmt.Sprintf(query, "")
		db.Raw(query).Scan(&couponUsage)
	}
	data.CouponUsage = couponUsage
}

func getMonthlyRevenue(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	var monthlyRevenue []MonthlyRevenuePoint
	query := `
		SELECT 
			TO_CHAR(created_at, 'YYYY-MM') as month,
			COALESCE(SUM(total_price), 0) as revenue
		FROM orders 
		WHERE payment_status = 'Paid'
		%s
		GROUP BY TO_CHAR(created_at, 'YYYY-MM')
		ORDER BY month ASC
	`
	var whereClause string
	if useCustomRange {
		whereClause = "AND created_at >= ? AND created_at < ?"
		query = fmt.Sprintf(query, whereClause)
		db.Raw(query, startDate, endDate).Scan(&monthlyRevenue)
	} else {
		query = fmt.Sprintf(query, "AND created_at >= ?")
		startDate := time.Now().AddDate(-1, 0, 0)
		db.Raw(query, startDate).Scan(&monthlyRevenue)
	}
	data.MonthlyRevenue = monthlyRevenue
}

func ExportSalesReport(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")
	filter := c.DefaultQuery("filter", "monthly")
	preview := c.DefaultQuery("preview", "false") // Add preview parameter
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	db := database.GetDB()
	var salesData []SalesReportData
	var query string
	var args []interface{}

	if startDateStr != "" && endDateStr != "" {
		startDate, err1 := time.Parse("2006-01-02", startDateStr)
		endDate, err2 := time.Parse("2006-01-02", endDateStr)
		if err1 == nil && err2 == nil {
			query = `
				SELECT 
					DATE(o.created_at)::text as date,
					COUNT(DISTINCT o.id) as orders,
					COALESCE(SUM(o.total_price), 0) as revenue,
					COALESCE(SUM(oi.quantity), 0) as products_sold,
					COALESCE(SUM(o.coupon_discount), 0) as coupon_discount,
					COALESCE(SUM(o.total_discount - o.coupon_discount), 0) as offer_discount,
					COALESCE(SUM(o.total_discount), 0) as total_discount,
					STRING_AGG(DISTINCT o.coupon_code, ', ') FILTER (WHERE o.coupon_code IS NOT NULL) as coupon_codes
				FROM orders o
				LEFT JOIN order_items oi ON o.id = oi.order_id
				WHERE o.created_at >= $1 AND o.created_at <= $2
					AND o.status NOT IN ('cancelled', 'refunded')
				GROUP BY DATE(o.created_at)
				ORDER BY date ASC
			`
			args = []interface{}{startDate, endDate.AddDate(0, 0, 1)}
		}
	}

	if query == "" {
		var interval string
		switch filter {
		case "daily":
			interval = "1 day"
		case "weekly":
			interval = "7 days"
		case "monthly":
			interval = "1 month"
		case "yearly":
			interval = "1 year"
		default:
			interval = "1 month"
		}

		query = `
			SELECT 
				DATE(o.created_at)::text as date,
				COUNT(DISTINCT o.id) as orders,
				COALESCE(SUM(o.total_price), 0) as revenue,
				COALESCE(SUM(oi.quantity), 0) as products_sold,
				COALESCE(SUM(o.coupon_discount), 0) as coupon_discount,
				COALESCE(SUM(o.total_discount - o.coupon_discount), 0) as offer_discount,
				COALESCE(SUM(o.total_discount), 0) as total_discount,
				STRING_AGG(DISTINCT o.coupon_code, ', ') FILTER (WHERE o.coupon_code IS NOT NULL) as coupon_codes
			FROM orders o
			LEFT JOIN order_items oi ON o.id = oi.order_id
			WHERE o.created_at >= NOW() - INTERVAL '` + interval + `' 
				AND o.status NOT IN ('cancelled', 'refunded')
			GROUP BY DATE(o.created_at)
			ORDER BY date ASC
		`
	}

	db.Raw(query, args...).Scan(&salesData)

	var totalOrders, totalProducts int64
	var totalRevenue, totalDiscount, totalCouponDiscount, totalOfferDiscount float64
	for _, record := range salesData {
		totalOrders += record.Orders
		totalProducts += record.ProductsSold
		totalRevenue += record.Revenue
		totalDiscount += record.TotalDiscount
		totalCouponDiscount += record.CouponDiscount
		totalOfferDiscount += record.OfferDiscount
	}

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=sales_report_"+time.Now().Format("20060102")+".csv")
		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		writer.Write([]string{"Date", "Orders", "Revenue ($)", "Products Sold", "Total Discount ($)", "Coupon Discount ($)", "Offer Discount ($)", "Coupon Codes"})
		for _, record := range salesData {
			writer.Write([]string{
				record.Date,
				strconv.FormatInt(record.Orders, 10),
				fmt.Sprintf("%.2f", record.Revenue),
				strconv.FormatInt(record.ProductsSold, 10),
				fmt.Sprintf("%.2f", record.TotalDiscount),
				fmt.Sprintf("%.2f", record.CouponDiscount),
				fmt.Sprintf("%.2f", record.OfferDiscount),
				record.CouponCodes,
			})
		}
		writer.Write([]string{})
		writer.Write([]string{"TOTAL", "", "", "", "", "", "", ""})
		writer.Write([]string{
			"",
			strconv.FormatInt(totalOrders, 10),
			fmt.Sprintf("%.2f", totalRevenue),
			strconv.FormatInt(totalProducts, 10),
			fmt.Sprintf("%.2f", totalDiscount),
			fmt.Sprintf("%.2f", totalCouponDiscount),
			fmt.Sprintf("%.2f", totalOfferDiscount),
			"",
		})

	case "pdf":
		cfg := config.NewBuilder().Build()
		m := maroto.New(cfg)

		// Title row
		m.AddRows(
			text.NewRow(20, "Sales Report", props.Text{
				Top:   2,
				Size:  14,
				Style: fontstyle.Bold,
				Align: align.Center,
			}),

			text.NewRow(10, fmt.Sprintf("Filter: %s", filter), props.Text{
				Align: "center",
			}),
		)

		// Header row
		m.AddRows(
			row.New(10).Add(
				text.NewCol(2, "Date", props.Text{Style: fontstyle.Bold}),
				text.NewCol(2, "Orders", props.Text{Style: fontstyle.Bold}),
				text.NewCol(2, "Revenue ($)", props.Text{Style: fontstyle.Bold}),
				text.NewCol(2, "Products Sold", props.Text{Style: fontstyle.Bold}),
				text.NewCol(2, "Total Discount ($)", props.Text{Style: fontstyle.Bold}),
				text.NewCol(2, "Coupon Codes", props.Text{Style: fontstyle.Bold}),
			),
		)

		// Data rows
		for _, record := range salesData {
			m.AddRows(
				row.New(8).Add(
					text.NewCol(2, record.Date),
					text.NewCol(2, strconv.FormatInt(record.Orders, 10)),
					text.NewCol(2, fmt.Sprintf("%.2f", record.Revenue)),
					text.NewCol(2, strconv.FormatInt(record.ProductsSold, 10)),
					text.NewCol(2, fmt.Sprintf("%.2f", record.TotalDiscount)),
					text.NewCol(2, record.CouponCodes),
				),
			)
		}

		// Summary row
		m.AddRows(
			row.New(8),
			row.New(8).Add(
				text.NewCol(2, "TOTAL", props.Text{Style: fontstyle.Bold}),
				text.NewCol(2, strconv.FormatInt(totalOrders, 10)),
				text.NewCol(2, fmt.Sprintf("%.2f", totalRevenue)),
				text.NewCol(2, strconv.FormatInt(totalProducts, 10)),
				text.NewCol(2, fmt.Sprintf("%.2f", totalDiscount)),
				text.NewCol(2, ""),
			),
		)

		pdf, err := m.Generate()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to generate PDF"})
			return
		}

		c.Header("Content-Type", "application/pdf")

		// Check if it's preview mode
		if preview == "true" {
			// For preview - display inline in browser
			c.Header("Content-Disposition", "inline; filename=sales_report_"+time.Now().Format("20060102")+".pdf")
		} else {
			// For download - force download
			c.Header("Content-Disposition", "attachment; filename=sales_report_"+time.Now().Format("20060102")+".pdf")
		}

		c.Data(http.StatusOK, "application/pdf", pdf.GetBytes())

	default:
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   salesData,
			"summary": gin.H{
				"total_orders":          totalOrders,
				"total_revenue":         totalRevenue,
				"total_products_sold":   totalProducts,
				"total_discount":        totalDiscount,
				"total_coupon_discount": totalCouponDiscount,
				"total_offer_discount":  totalOfferDiscount,
			},
			"filter": filter,
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
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Action logged successfully",
	})
}
