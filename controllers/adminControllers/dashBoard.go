package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DashboardData struct {
	TotalUsers      int64   `json:"total_users"`
	TotalOrders     int64   `json:"total_orders"`
	TotalRevenue    float64 `json:"total_revenue"`
	TotalProducts   int64   `json:"total_products"`
	ActiveCoupons   int64   `json:"active_coupons"`
	PendingOrders   int64   `json:"pending_orders"`
	CompletedOrders int64   `json:"completed_orders"`
	CancelledOrders int64   `json:"cancelled_orders"`
	// Remove AvgOrderValue and replace with:
	TotalDiscount    float64                 `json:"total_discount"` // New field for overall discount
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
	query = db.Model(&userModels.Orders{}).
		Where("(payment_status = ? OR (payment_method = ? AND status = ?))",
			"Paid", "COD", "Delivered").
		Where("status NOT IN ?", []string{"Cancelled", "Returned", "Return Rejected"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Select("COALESCE(SUM(total_price), 0.0) as total").Scan(&revenue)
	dashboardData.TotalRevenue = revenue.Total

	dashboardData.TotalDiscount = calculateTotalDiscount(db, useCustomRange, startDate, endDate)

	getTopProducts(db, &dashboardData, useCustomRange, startDate, endDate)
	getTopBrands(db, &dashboardData, useCustomRange, startDate, endDate)
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
		// Added: Same default range logic as ShowDashboard
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
	query = db.Model(&userModels.Orders{}).
		Where("(payment_status = ? OR (payment_method = ? AND status = ?))",
			"Paid", "COD", "Delivered").
		Where("status NOT IN ?", []string{"Cancelled", "Returned", "Return Rejected"})
	if useCustomRange {
		query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}
	query.Select("COALESCE(SUM(total_price), 0.0) as total").Scan(&revenue)
	dashboardData.TotalRevenue = revenue.Total
	dashboardData.TotalDiscount = calculateTotalDiscount(db, useCustomRange, startDate, endDate)

	getTopProducts(db, &dashboardData, useCustomRange, startDate, endDate)
	getTopBrands(db, &dashboardData, useCustomRange, startDate, endDate)
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
		db.Raw(query, startDate, endDate).Scan(&topProducts)
	} else {
		query = fmt.Sprintf(query, "")
		db.Raw(query).Scan(&topProducts)
	}
	data.TopProducts = topProducts
}

func getTopBrands(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	var topBrands []TopBrand
	query := `
		SELECT 
			COALESCE(NULLIF(p.brand, ''), 'Unknown Brand') AS brand_name,  -- Handle empty brands
			COALESCE(SUM(oi.quantity), 0) as total_sold,
			COALESCE(SUM(oi.item_total), 0) as revenue
		FROM products p
		LEFT JOIN order_items oi ON p.id = oi.product_id
		LEFT JOIN orders o ON oi.order_id = o.id
		WHERE (o.status NOT IN ('Cancelled', 'Returned', 'Return Rejected') OR o.status IS NULL)
		AND p.brand IS NOT NULL AND p.brand != ''  -- Exclude empty brands
		%s
		GROUP BY p.brand
		HAVING COALESCE(SUM(oi.quantity), 0) > 0  -- Only brands with sales
		ORDER BY total_sold DESC
		LIMIT 5
	`
	var whereClause string
	if useCustomRange {
		whereClause = "AND o.created_at >= ? AND o.created_at < ?"
		query = fmt.Sprintf(query, whereClause)
		db.Raw(query, startDate, endDate).Scan(&topBrands)
	} else {
		query = fmt.Sprintf(query, "")
		db.Raw(query).Scan(&topBrands)
	}
	data.TopBrands = topBrands
	if len(topBrands) == 0 {
		// Fallback for no data
		data.TopBrands = []TopBrand{{BrandName: "No Brands Sold", TotalSold: 0, Revenue: 0}}
	}
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
        WHERE (payment_status = ? OR (payment_method = ? AND status = ?))
        AND status NOT IN ('Cancelled', 'Returned', 'Return Rejected')
        %s
        GROUP BY %s
        ORDER BY date_group ASC
    `
	var whereClause string
	if useCustomRange {
		whereClause = "AND created_at >= ? AND created_at < ?"
		query = fmt.Sprintf(query, groupBy, whereClause, groupBy)
		db.Raw(query, "Paid", "COD", "Delivered", startDate, endDate).Scan(&salesData)
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
		db.Raw(query, "Paid", "COD", "Delivered").Scan(&salesData)
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
        
        // Fix the subquery condition
        subQueryCondition := groupBy
        if filter == "daily" {
            subQueryCondition = "DATE(us.created_at)"
        } else if filter == "weekly" {
            subQueryCondition = "DATE_TRUNC('week', us.created_at)"
        } else if filter == "monthly" {
            subQueryCondition = "DATE_TRUNC('month', us.created_at)"
        } else if filter == "yearly" {
            subQueryCondition = "DATE_TRUNC('year', us.created_at)"
        }
        
        query = fmt.Sprintf(query, subQueryCondition, groupBy, startExpr, endExpr, groupBy, whereClause)
        db.Raw(query, startDate, endDate, startDate, endDate, startDate, endDate).Scan(&userActivity)
    } else {
        startExpr = "NOW() - INTERVAL '" + interval + "'"
        endExpr = "NOW()"
        
        // Fix the subquery condition for default case
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
        db.Raw(query).Scan(&userActivity)
    }
    
    // Format dates for display
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
        }
    }
    data.UserActivityData = userActivity
}
func getInventoryStatus(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	// Always fetch current inventory status, no date filtering
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

func getCouponUsage(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
	// Always fetch all-time usage for active coupons, no date filtering
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
func getMonthlyRevenue(db *gorm.DB, data *DashboardData, useCustomRange bool, startDate, endDate time.Time) {
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
		db.Raw(query, "Paid", "COD", "Delivered", startDate, endDate).Scan(&monthlyRevenue)
	} else {
		startDate := time.Now().AddDate(0, -12, 0)
		query = fmt.Sprintf(query, "AND created_at >= ?")
		db.Raw(query, "Paid", "COD", "Delivered", startDate).Scan(&monthlyRevenue)
	}
	data.MonthlyRevenue = monthlyRevenue
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

func calculateTotalDiscount(db *gorm.DB, useCustomRange bool, startDate, endDate time.Time) float64 {
	var totalDiscount struct {
		CouponDiscount float64
		OfferDiscount  float64
	}

	// Calculate coupon discount
	couponQuery := db.Model(&userModels.Orders{}).
		Where("(payment_status = ? OR (payment_method = ? AND status = ?))",
			"Paid", "COD", "Delivered").
		Where("status NOT IN ?", []string{"Cancelled", "Returned", "Return Rejected"})

	if useCustomRange {
		couponQuery = couponQuery.Where("created_at >= ? AND created_at < ?", startDate, endDate)
	}

	// Fixed: Use the exact struct field name in the alias
	if err := couponQuery.Select("COALESCE(SUM(coupon_discount), 0.0) as coupon_discount").Scan(&totalDiscount).Error; err != nil {
		// Log error or handle it
		return 0
	}

	// Calculate offer discount (from order_items)
	offerQuery := db.Model(&userModels.OrderItem{}).
		Joins("JOIN orders ON order_items.order_id = orders.id").
		Where("(orders.payment_status = ? OR (orders.payment_method = ? AND orders.status = ?))",
			"Paid", "COD", "Delivered").
		Where("orders.status NOT IN ?", []string{"Cancelled", "Returned", "Return Rejected"})

	if useCustomRange {
		offerQuery = offerQuery.Where("orders.created_at >= ? AND orders.created_at < ?", startDate, endDate)
	}

	// Fixed: Use the exact struct field name in the alias
	if err := offerQuery.Select("COALESCE(SUM(order_items.discount_amount), 0.0) as offer_discount").Scan(&totalDiscount).Error; err != nil {
		// Log error or handle it
		return 0
	}

	return totalDiscount.CouponDiscount + totalDiscount.OfferDiscount
}
