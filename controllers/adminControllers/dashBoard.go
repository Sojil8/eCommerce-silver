package controllers

import (
	"encoding/json"
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
	TotalDiscount    float64                 `json:"total_discount"`
	TopProducts      []TopProduct            `json:"top_products"`
	TopBrands        []TopBrand              `json:"top_brands"`
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

type TopBrand struct {
	BrandName string  `json:"brand_name"`
	TotalSold int64   `json:"total_sold"`
	Revenue   float64 `json:"revenue"`
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
	pkg.Log.Info("Handling request to show dashboard")

	filter := c.DefaultQuery("filter", "weekly")
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	pkg.Log.Debug("Dashboard parameters", zap.String("filter", filter), zap.String("start_date", startDateStr), zap.String("end_date", endDateStr))

	var dashboardData DashboardData
	db := database.GetDB()

	var startDate, endDate time.Time
	var useCustomRange bool
	if startDateStr != "" && endDateStr != "" {
		var err1, err2 error
		startDate, err1 = time.Parse("2006-01-02", startDateStr)
		endDate, err2 = time.Parse("2006-01-02", endDateStr)
		if err1 != nil || err2 != nil {
			pkg.Log.Error("Failed to parse custom date range",
				zap.String("start_date", startDateStr),
				zap.String("end_date", endDateStr),
				zap.Error(err1),
				zap.Error(err2))
			helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid date format", "Start and end dates must be in YYYY-MM-DD format", "")
			return
		}
		useCustomRange = true
		endDate = endDate.AddDate(0, 0, 1)
		pkg.Log.Debug("Using custom date range", zap.Time("start_date", startDate), zap.Time("end_date", endDate))
	} else {
		useCustomRange = true
		now := time.Now()
		endDate = now.AddDate(0, 0, 1)
		switch filter {
		case "daily":
			startDate = now.AddDate(0, 0, -7)
		case "weekly":
			startDate = now.AddDate(0, 0, -28)
		case "monthly":
			startDate = now.AddDate(0, -12, 0)
		case "yearly":
			startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		default:
			useCustomRange = false
		}
		pkg.Log.Debug("Using default date range for filter", zap.String("filter", filter), zap.Time("start_date", startDate), zap.Time("end_date", endDate))
	}

	query := db.Model(&userModels.Users{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.TotalUsers).Error; err != nil {
		pkg.Log.Error("Failed to count total users", zap.Error(err))
	}
	pkg.Log.Debug("Total users counted", zap.Int64("total_users", dashboardData.TotalUsers))

	query = db.Model(&userModels.Orders{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.TotalOrders).Error; err != nil {
		pkg.Log.Error("Failed to count total orders", zap.Error(err))
	}
	pkg.Log.Debug("Total orders counted", zap.Int64("total_orders", dashboardData.TotalOrders))

	query = db.Model(&adminModels.Product{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.TotalProducts).Error; err != nil {
		pkg.Log.Error("Failed to count total products", zap.Error(err))
	}
	pkg.Log.Debug("Total products counted", zap.Int64("total_products", dashboardData.TotalProducts))

	query = db.Model(&adminModels.Coupons{}).Where("is_active = ?", true)
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.ActiveCoupons).Error; err != nil {
		pkg.Log.Error("Failed to count active coupons", zap.Error(err))
	}
	pkg.Log.Debug("Active coupons counted", zap.Int64("active_coupons", dashboardData.ActiveCoupons))

	query = db.Model(&userModels.Orders{}).Where("status IN ?", []string{"Pending", "Confirmed", "Shipped", "Out for Delivery"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.PendingOrders).Error; err != nil {
		pkg.Log.Error("Failed to count pending orders", zap.Error(err))
	}
	pkg.Log.Debug("Pending orders counted", zap.Int64("pending_orders", dashboardData.PendingOrders))

	query = db.Model(&userModels.Orders{}).Where("status = ?", "Delivered")
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.CompletedOrders).Error; err != nil {
		pkg.Log.Error("Failed to count completed orders", zap.Error(err))
	}
	pkg.Log.Debug("Completed orders counted", zap.Int64("completed_orders", dashboardData.CompletedOrders))

	query = db.Model(&userModels.Orders{}).Where("status IN ?", []string{"Return Rejected", "Cancelled", "Returned"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.CancelledOrders).Error; err != nil {
		pkg.Log.Error("Failed to count cancelled orders", zap.Error(err))
	}
	pkg.Log.Debug("Cancelled orders counted", zap.Int64("cancelled_orders", dashboardData.CancelledOrders))

	var revenue struct {
		Total float64
	}
	query = db.Model(&userModels.Orders{}).
		Where("(payment_status = ? OR (payment_method = ? AND status = ?))",
			"Paid", "COD", "Delivered").
		Where("status NOT IN ?", []string{"Cancelled", "Returned", "Return Rejected"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Select("COALESCE(SUM(total_price), 0.0) as total").Scan(&revenue).Error; err != nil {
		pkg.Log.Error("Failed to calculate total revenue", zap.Error(err))
	}
	dashboardData.TotalRevenue = revenue.Total
	pkg.Log.Debug("Total revenue calculated", zap.Float64("total_revenue", dashboardData.TotalRevenue))

	dashboardData.TotalDiscount = calculateTotalDiscount(db, useCustomRange, startDate, endDate)
	pkg.Log.Debug("Total discount calculated", zap.Float64("total_discount", dashboardData.TotalDiscount))

	getTopProducts(db, &dashboardData, useCustomRange, startDate, endDate)
	getTopBrands(db, &dashboardData, useCustomRange, startDate, endDate)
	getTopCategories(db, &dashboardData, useCustomRange, startDate, endDate)
	getRecentOrders(db, &dashboardData, useCustomRange, startDate, endDate)
	getSalesData(db, &dashboardData, filter, useCustomRange, startDate, endDate)
	getUserActivityData(db, &dashboardData, filter, useCustomRange, startDate, endDate)
	getInventoryStatus(db, &dashboardData, useCustomRange, startDate, endDate)
	getCouponUsage(db, &dashboardData, useCustomRange, startDate, endDate)
	getMonthlyRevenue(db, &dashboardData, useCustomRange, startDate, endDate)

	topProductsJSON, err := json.Marshal(dashboardData.TopProducts)
	if err != nil {
		pkg.Log.Error("Failed to marshal top products", zap.Error(err))
	}
	topCategoriesJSON, err := json.Marshal(dashboardData.TopCategories)
	if err != nil {
		pkg.Log.Error("Failed to marshal top categories", zap.Error(err))
	}
	recentOrdersJSON, err := json.Marshal(dashboardData.RecentOrders)
	if err != nil {
		pkg.Log.Error("Failed to marshal recent orders", zap.Error(err))
	}
	salesDataJSON, err := json.Marshal(dashboardData.SalesData)
	if err != nil {
		pkg.Log.Error("Failed to marshal sales data", zap.Error(err))
	}
	userActivityDataJSON, err := json.Marshal(dashboardData.UserActivityData)
	if err != nil {
		pkg.Log.Error("Failed to marshal user activity data", zap.Error(err))
	}
	inventoryStatusJSON, err := json.Marshal(dashboardData.InventoryStatus)
	if err != nil {
		pkg.Log.Error("Failed to marshal inventory status", zap.Error(err))
	}
	couponUsageJSON, err := json.Marshal(dashboardData.CouponUsage)
	if err != nil {
		pkg.Log.Error("Failed to marshal coupon usage", zap.Error(err))
	}
	monthlyRevenueJSON, err := json.Marshal(dashboardData.MonthlyRevenue)
	if err != nil {
		pkg.Log.Error("Failed to marshal monthly revenue", zap.Error(err))
	}

	pkg.Log.Info("Rendering dashboard.html",
		zap.Int64("total_users", dashboardData.TotalUsers),
		zap.Int64("total_orders", dashboardData.TotalOrders),
		zap.Float64("total_revenue", dashboardData.TotalRevenue),
		zap.Int("top_products_count", len(dashboardData.TopProducts)))
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
		"TotalDiscount":        dashboardData.TotalDiscount,
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
	pkg.Log.Info("Handling request to get dashboard data")

	filter := c.DefaultQuery("filter", "weekly")
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	pkg.Log.Debug("Dashboard data parameters", zap.String("filter", filter), zap.String("start_date", startDateStr), zap.String("end_date", endDateStr))

	var dashboardData DashboardData
	db := database.GetDB()

	var startDate, endDate time.Time
	var useCustomRange bool
	if startDateStr != "" && endDateStr != "" {
		var err1, err2 error
		startDate, err1 = time.Parse("2006-01-02", startDateStr)
		endDate, err2 = time.Parse("2006-01-02", endDateStr)
		if err1 != nil || err2 != nil {
			pkg.Log.Error("Failed to parse custom date range",
				zap.String("start_date", startDateStr),
				zap.String("end_date", endDateStr),
				zap.Error(err1),
				zap.Error(err2))
			helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid date format", "Start and end dates must be in YYYY-MM-DD format", "")
			return
		}
		useCustomRange = true
		endDate = endDate.AddDate(0, 0, 1)
		pkg.Log.Debug("Using custom date range", zap.Time("start_date", startDate), zap.Time("end_date", endDate))
	} else {
		useCustomRange = true
		now := time.Now()
		endDate = now.AddDate(0, 0, 1)
		switch filter {
		case "daily":
			startDate = now.AddDate(0, 0, -7)
		case "weekly":
			startDate = now.AddDate(0, 0, -28)
		case "monthly":
			startDate = now.AddDate(0, -12, 0)
		case "yearly":
			startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		default:
			useCustomRange = false
		}
		pkg.Log.Debug("Using default date range for filter", zap.String("filter", filter), zap.Time("start_date", startDate), zap.Time("end_date", endDate))
	}

	query := db.Model(&userModels.Users{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.TotalUsers).Error; err != nil {
		pkg.Log.Error("Failed to count total users", zap.Error(err))
	}
	pkg.Log.Debug("Total users counted", zap.Int64("total_users", dashboardData.TotalUsers))

	query = db.Model(&userModels.Orders{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.TotalOrders).Error; err != nil {
		pkg.Log.Error("Failed to count total orders", zap.Error(err))
	}
	pkg.Log.Debug("Total orders counted", zap.Int64("total_orders", dashboardData.TotalOrders))

	query = db.Model(&adminModels.Product{})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.TotalProducts).Error; err != nil {
		pkg.Log.Error("Failed to count total products", zap.Error(err))
	}
	pkg.Log.Debug("Total products counted", zap.Int64("total_products", dashboardData.TotalProducts))

	query = db.Model(&adminModels.Coupons{}).Where("is_active = ?", true)
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.ActiveCoupons).Error; err != nil {
		pkg.Log.Error("Failed to count active coupons", zap.Error(err))
	}
	pkg.Log.Debug("Active coupons counted", zap.Int64("active_coupons", dashboardData.ActiveCoupons))

	query = db.Model(&userModels.Orders{}).Where("status IN ?", []string{"Pending", "Confirmed", "Shipped", "Out for Delivery"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.PendingOrders).Error; err != nil {
		pkg.Log.Error("Failed to count pending orders", zap.Error(err))
	}
	pkg.Log.Debug("Pending orders counted", zap.Int64("pending_orders", dashboardData.PendingOrders))

	query = db.Model(&userModels.Orders{}).Where("status = ?", "Delivered")
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.CompletedOrders).Error; err != nil {
		pkg.Log.Error("Failed to count completed orders", zap.Error(err))
	}
	pkg.Log.Debug("Completed orders counted", zap.Int64("completed_orders", dashboardData.CompletedOrders))

	query = db.Model(&userModels.Orders{}).Where("status IN ?", []string{"Return Rejected", "Cancelled", "Returned"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Count(&dashboardData.CancelledOrders).Error; err != nil {
		pkg.Log.Error("Failed to count cancelled orders", zap.Error(err))
	}
	pkg.Log.Debug("Cancelled orders counted", zap.Int64("cancelled_orders", dashboardData.CancelledOrders))

	var revenue struct {
		Total float64
	}
	query = db.Model(&userModels.Orders{}).
		Where("(payment_status = ? OR (payment_method = ? AND status = ?))",
			"Paid", "COD", "Delivered").
		Where("status NOT IN ?", []string{"Cancelled", "Returned", "Return Rejected"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Select("COALESCE(SUM(total_price), 0.0) as total").Scan(&revenue).Error; err != nil {
		pkg.Log.Error("Failed to calculate total revenue", zap.Error(err))
	}
	dashboardData.TotalRevenue = revenue.Total
	pkg.Log.Debug("Total revenue calculated", zap.Float64("total_revenue", dashboardData.TotalRevenue))

	dashboardData.TotalDiscount = calculateTotalDiscount(db, useCustomRange, startDate, endDate)
	pkg.Log.Debug("Total discount calculated", zap.Float64("total_discount", dashboardData.TotalDiscount))

	getTopProducts(db, &dashboardData, useCustomRange, startDate, endDate)
	getTopBrands(db, &dashboardData, useCustomRange, startDate, endDate)
	getTopCategories(db, &dashboardData, useCustomRange, startDate, endDate)
	getRecentOrders(db, &dashboardData, useCustomRange, startDate, endDate)
	getSalesData(db, &dashboardData, filter, useCustomRange, startDate, endDate)
	getUserActivityData(db, &dashboardData, filter, useCustomRange, startDate, endDate)
	getInventoryStatus(db, &dashboardData, useCustomRange, startDate, endDate)
	getCouponUsage(db, &dashboardData, useCustomRange, startDate, endDate)
	getMonthlyRevenue(db, &dashboardData, useCustomRange, startDate, endDate)

	pkg.Log.Info("Returning dashboard data",
		zap.Int64("total_users", dashboardData.TotalUsers),
		zap.Int64("total_orders", dashboardData.TotalOrders),
		zap.Float64("total_revenue", dashboardData.TotalRevenue))
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   dashboardData,
	})
}

func getTopProducts(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	pkg.Log.Debug("Fetching top products")
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
        WHERE (o.status NOT IN ('Cancelled', 'Returned', 'Return Rejected') OR o.status IS NULL)
        AND (oi.status IS NULL OR oi.status != 'Cancelled')
        %s
        GROUP BY p.id, p.product_name, p.category_name
        ORDER BY total_sold DESC
        LIMIT 10
    `
	var whereClause string
	if useCustomRange {
		whereClause = "AND o.created_at >= ? AND o.created_at < ?"
		query = fmt.Sprintf(query, whereClause)
		if err := db.Raw(query, startDate, endDate).Scan(&topProducts).Error; err != nil {
			pkg.Log.Error("Failed to fetch top products", zap.Error(err))
		}
	} else {
		query = fmt.Sprintf(query, "")
		if err := db.Raw(query).Scan(&topProducts).Error; err != nil {
			pkg.Log.Error("Failed to fetch top products", zap.Error(err))
		}
	}
	data.TopProducts = topProducts
	pkg.Log.Debug("Top products retrieved", zap.Int("count", len(topProducts)))
}

func getTopBrands(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	pkg.Log.Debug("Fetching top brands")
	var topBrands []TopBrand
	query := `
        SELECT 
            COALESCE(NULLIF(p.brand, ''), 'Unknown Brand') AS brand_name,
            COALESCE(SUM(oi.quantity), 0) as total_sold,
            COALESCE(SUM(oi.item_total), 0) as revenue
        FROM products p
        LEFT JOIN order_items oi ON p.id = oi.product_id
        LEFT JOIN orders o ON oi.order_id = o.id
        WHERE (o.status NOT IN ('Cancelled', 'Returned', 'Return Rejected') OR o.status IS NULL)
        AND p.brand IS NOT NULL AND p.brand != ''
        %s
        GROUP BY p.brand
        HAVING COALESCE(SUM(oi.quantity), 0) > 0
        ORDER BY total_sold DESC
        LIMIT 5
    `
	var whereClause string
	if useCustomRange {
		whereClause = "AND o.created_at >= ? AND o.created_at < ?"
		query = fmt.Sprintf(query, whereClause)
		if err := db.Raw(query, startDate, endDate).Scan(&topBrands).Error; err != nil {
			pkg.Log.Error("Failed to fetch top brands", zap.Error(err))
		}
	} else {
		query = fmt.Sprintf(query, "")
		if err := db.Raw(query).Scan(&topBrands).Error; err != nil {
			pkg.Log.Error("Failed to fetch top brands", zap.Error(err))
		}
	}
	data.TopBrands = topBrands
	if len(topBrands) == 0 {
		data.TopBrands = []TopBrand{{BrandName: "No Brands Sold", TotalSold: 0, Revenue: 0}}
	}
	pkg.Log.Debug("Top brands retrieved", zap.Int("count", len(topBrands)))
}

func getTopCategories(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	pkg.Log.Debug("Fetching top categories")
	var topCategories []TopCategory
	query := `
        SELECT 
            p.category_name,
            COALESCE(SUM(oi.quantity), 0) as total_sold,
            COALESCE(SUM(oi.item_total), 0) as revenue
        FROM products p
        LEFT JOIN order_items oi ON p.id = oi.product_id
        LEFT JOIN orders o ON oi.order_id = o.id
        WHERE (o.status NOT IN ('Cancelled', 'Returned', 'Return Rejected') OR o.status IS NULL)
        %s
        GROUP BY p.category_name
        ORDER BY total_sold DESC
        LIMIT 10
    `
	var whereClause string
	if useCustomRange {
		whereClause = "AND o.created_at >= ? AND o.created_at < ?"
		query = fmt.Sprintf(query, whereClause)
		if err := db.Raw(query, startDate, endDate).Scan(&topCategories).Error; err != nil {
			pkg.Log.Error("Failed to fetch top categories", zap.Error(err))
		}
	} else {
		query = fmt.Sprintf(query, "")
		if err := db.Raw(query).Scan(&topCategories).Error; err != nil {
			pkg.Log.Error("Failed to fetch top categories", zap.Error(err))
		}
	}
	data.TopCategories = topCategories
	pkg.Log.Debug("Top categories retrieved", zap.Int("count", len(topCategories)))
}

func getRecentOrders(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	pkg.Log.Debug("Fetching recent orders")
	var recentOrders []userModels.Orders
	query := db.Preload("User").Preload("OrderItems").Preload("OrderItems.Product").Order("created_at DESC").Limit(10)
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := query.Find(&recentOrders).Error; err != nil {
		pkg.Log.Error("Failed to fetch recent orders", zap.Error(err))
	}
	data.RecentOrders = recentOrders
	pkg.Log.Debug("Recent orders retrieved", zap.Int("count", len(recentOrders)))
}

func getSalesData(db *gorm.DB, data *DashboardData, filter string, useCustomRange bool, startDate, endDate time.Time) {
	pkg.Log.Debug("Fetching sales data", zap.String("filter", filter))
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
            %s as date,
            COALESCE(SUM(total_price), 0) as sales,
            COUNT(*) as orders
        FROM orders 
        WHERE (payment_status = ? OR (payment_method = ? AND status = ?))
        AND status NOT IN ('Cancelled', 'Returned', 'Return Rejected')
        %s
        GROUP BY %s
        ORDER BY date ASC
    `
	var whereClause string
	if useCustomRange {
		whereClause = "AND created_at >= ? AND created_at < ?"
		query = fmt.Sprintf(query, groupBy, whereClause, groupBy)
		if err := db.Raw(query, "Paid", "COD", "Delivered", startDate, endDate).Scan(&salesData).Error; err != nil {
			pkg.Log.Error("Failed to fetch sales data", zap.String("filter", filter), zap.Error(err))
		}
	} else {
		var interval string
		switch filter {
		case "daily":
			interval = "7 days"
		case "weekly":
			interval = "28 days"
		case "monthly":
			interval = "12 months"
		case "yearly":
			interval = "1 year"
		default:
			interval = "90 days"
		}
		whereClause = "AND created_at >= NOW() - INTERVAL '" + interval + "'"
		query = fmt.Sprintf(query, groupBy, whereClause, groupBy)
		if err := db.Raw(query, "Paid", "COD", "Delivered").Scan(&salesData).Error; err != nil {
			pkg.Log.Error("Failed to fetch sales data", zap.String("filter", filter), zap.Error(err))
		}
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
		} else {
			pkg.Log.Warn("Failed to parse date in sales data", zap.String("date", salesData[i].Date), zap.Error(err))
		}
	}
	data.SalesData = salesData
	pkg.Log.Debug("Sales data retrieved", zap.Int("count", len(salesData)))
}

func getUserActivityData(db *gorm.DB, data *DashboardData, filter string, useCustomRange bool, startDate, endDate time.Time) {
	pkg.Log.Debug("Fetching user activity data", zap.String("filter", filter))
	var userActivity []UserActivityDataPoint
	var interval string
	var groupBy string

	switch filter {
	case "daily":
		interval = "7 DAY"
		groupBy = "DATE(u.created_at)"
	case "weekly":
		interval = "4 WEEK"
		groupBy = "DATE_TRUNC('week', u.created_at)"
	case "monthly":
		interval = "12 MONTH"
		groupBy = "DATE_TRUNC('month', u.created_at)"
	case "yearly":
		interval = "1 YEAR"
		groupBy = "DATE_TRUNC('year', u.created_at)"
	default:
		interval = "4 WEEK"
		groupBy = "DATE_TRUNC('week', u.created_at)"
	}

	query := `
        SELECT 
            dates.date as date,
            COUNT(u.id) as new_users,
            (SELECT COUNT(DISTINCT user_id) FROM user_sessions us 
             WHERE %s = dates.date) as active_users
        FROM (
            SELECT %s as date
            FROM generate_series(
                %s,
                %s,
                INTERVAL '1 day'
            ) AS gs(date)
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
		subQueryCondition := groupBy
		switch filter {
		case "daily":
			subQueryCondition = "DATE(us.created_at)"
		case "weekly":
			subQueryCondition = "DATE_TRUNC('week', us.created_at)"
		case "monthly":
			subQueryCondition = "DATE_TRUNC('month', us.created_at)"
		case "yearly":
			subQueryCondition = "DATE_TRUNC('year', us.created_at)"
		}
		query = fmt.Sprintf(query, subQueryCondition, groupBy, startExpr, endExpr, groupBy, whereClause)
		if err := db.Raw(query, startDate, endDate, startDate, endDate, startDate, endDate).Scan(&userActivity).Error; err != nil {
			pkg.Log.Error("Failed to fetch user activity data", zap.String("filter", filter), zap.Error(err))
		}
	} else {
		startExpr = "NOW() - INTERVAL '" + interval + "'"
		endExpr = "NOW()"
		subQueryCondition := groupBy
		switch filter {
		case "daily":
			subQueryCondition = "DATE(us.created_at)"
		case "weekly":
			subQueryCondition = "DATE_TRUNC('week', us.created_at)"
		case "monthly":
			subQueryCondition = "DATE_TRUNC('month', us.created_at)"
		case "yearly":
			subQueryCondition = "DATE_TRUNC('year', us.created_at)"
		}
		query = fmt.Sprintf(query, subQueryCondition, groupBy, startExpr, endExpr, groupBy, "")
		if err := db.Raw(query).Scan(&userActivity).Error; err != nil {
			pkg.Log.Error("Failed to fetch user activity data", zap.String("filter", filter), zap.Error(err))
		}
	}

	for i := range userActivity {
		if t, err := time.Parse(time.RFC3339, userActivity[i].Date); err == nil {
			switch filter {
			case "daily":
				userActivity[i].Date = t.Format("Jan 02")
			case "weekly":
				userActivity[i].Date = t.Format("Jan 02")
			case "monthly":
				userActivity[i].Date = t.Format("Jan 2006")
			case "yearly":
				userActivity[i].Date = t.Format("2006")
			}
		} else {
			pkg.Log.Warn("Failed to parse date in user activity data", zap.String("date", userActivity[i].Date), zap.Error(err))
		}
	}
	data.UserActivityData = userActivity
	pkg.Log.Debug("User activity data retrieved", zap.Int("count", len(userActivity)))
}

func getInventoryStatus(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	pkg.Log.Debug("Fetching inventory status")
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
	if err := db.Raw(query).Scan(&inventory).Error; err != nil {
		pkg.Log.Error("Failed to fetch inventory status", zap.Error(err))
	}
	data.InventoryStatus = inventory
	pkg.Log.Debug("Inventory status retrieved", zap.Int("count", len(inventory)))
}

func getCouponUsage(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	pkg.Log.Debug("Fetching coupon usage")
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
	if err := db.Raw(query).Scan(&couponUsage).Error; err != nil {
		pkg.Log.Error("Failed to fetch coupon usage", zap.Error(err))
	}
	data.CouponUsage = couponUsage
	pkg.Log.Debug("Coupon usage retrieved", zap.Int("count", len(couponUsage)))
}

func getMonthlyRevenue(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	pkg.Log.Debug("Fetching monthly revenue")
	var monthlyRevenue []MonthlyRevenuePoint
	query := `
        SELECT 
            TO_CHAR(created_at, 'YYYY-MM') as month,
            COALESCE(SUM(total_price), 0) as revenue
        FROM orders 
        WHERE (payment_status = ? OR (payment_method = ? AND status = ?))
        AND status NOT IN ('Cancelled', 'Returned', 'Return Rejected')
        %s
        GROUP BY TO_CHAR(created_at, 'YYYY-MM')
        ORDER BY month ASC
    `
	var whereClause string
	if useCustomRange {
		whereClause = "AND created_at >= ? AND created_at < ?"
		query = fmt.Sprintf(query, whereClause)
		if err := db.Raw(query, "Paid", "COD", "Delivered", startDate, endDate).Scan(&monthlyRevenue).Error; err != nil {
			pkg.Log.Error("Failed to fetch monthly revenue", zap.Error(err))
		}
	} else {
		startDate := time.Now().AddDate(0, -12, 0)
		query = fmt.Sprintf(query, "AND created_at >= ?")
		if err := db.Raw(query, "Paid", "COD", "Delivered", startDate).Scan(&monthlyRevenue).Error; err != nil {
			pkg.Log.Error("Failed to fetch monthly revenue", zap.Error(err))
		}
	}
	data.MonthlyRevenue = monthlyRevenue
	pkg.Log.Debug("Monthly revenue retrieved", zap.Int("count", len(monthlyRevenue)))
}

func LogAdminAction(c *gin.Context) {
	pkg.Log.Info("Handling admin action log request")
	var actionLog struct {
		Action      string `json:"action" binding:"required"`
		Description string `json:"description"`
		EntityType  string `json:"entity_type"`
		EntityID    uint   `json:"entity_id"`
	}
	if err := c.ShouldBindJSON(&actionLog); err != nil {
		pkg.Log.Error("Failed to bind JSON data for action log", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", "Failed to bind data", "")
		return
	}
	pkg.Log.Info("Admin action logged",
		zap.String("action", actionLog.Action),
		zap.String("entity_type", actionLog.EntityType),
		zap.Uint("entity_id", actionLog.EntityID))
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Action logged successfully",
	})
}

func calculateTotalDiscount(db *gorm.DB, useCustomRange bool, startDate, endDate time.Time) float64 {
	pkg.Log.Debug("Calculating total discount")
	
	// Use separate variables for each query
	var couponDiscount struct {
		Total float64
	}
	var offerDiscount struct {
		Total float64
	}

	// Calculate coupon discount from Orders table
	couponQuery := db.Model(&userModels.Orders{}).
		Where("(payment_status = ? OR (payment_method = ? AND status = ?))",
			"Paid", "COD", "Delivered").
		Where("status NOT IN ?", []string{"Cancelled", "Returned", "Return Rejected"})
	if useCustomRange {
		couponQuery = couponQuery.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	if err := couponQuery.Select("COALESCE(SUM(coupon_discount), 0.0) as total").Scan(&couponDiscount).Error; err != nil {
		pkg.Log.Error("Failed to calculate coupon discount", zap.Error(err))
	}

	// Calculate offer discount from OrderItem table
	offerQuery := db.Model(&userModels.OrderItem{}).
		Joins("JOIN orders ON order_items.order_id = orders.id").
		Where("(orders.payment_status = ? OR (orders.payment_method = ? AND orders.status = ?))",
			"Paid", "COD", "Delivered").
		Where("orders.status NOT IN ?", []string{"Cancelled", "Returned", "Return Rejected"}).
		Where("order_items.status IS NULL OR order_items.status != 'Cancelled'")
	if useCustomRange {
		offerQuery = offerQuery.Where("orders.created_at >= ? AND orders.created_at < ?", startDate, endDate)
	}
	if err := offerQuery.Select("COALESCE(SUM(order_items.discount_amount), 0.0) as total").Scan(&offerDiscount).Error; err != nil {
		pkg.Log.Error("Failed to calculate offer discount", zap.Error(err))
	}

	total := couponDiscount.Total + offerDiscount.Total
	pkg.Log.Debug("Total discount calculated",
		zap.Float64("coupon_discount", couponDiscount.Total),
		zap.Float64("offer_discount", offerDiscount.Total),
		zap.Float64("total", total))
	return total
}