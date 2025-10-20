package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/gin-gonic/gin"
	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"github.com/xuri/excelize/v2"
)

type OrderDetail struct {
	OrderID  string  `json:"order_id"`
	Customer string  `json:"customer"`
	Date     string  `json:"date"`
	Amount   float64 `json:"amount"`
	Discount float64 `json:"discount"`
	Status   string  `json:"status"`
}

type ProductSales struct {
	ProductName string  `json:"product_name"`
	UnitsSold   int64   `json:"units_sold"`
	Revenue     float64 `json:"revenue"`
}

type CategorySales struct {
	CategoryName string  `json:"category_name"`
	UnitsSold    int64   `json:"units_sold"`
	Revenue      float64 `json:"revenue"`
}

type BrandSales struct {
	BrandName string  `json:"brand_name"`
	UnitsSold int64   `json:"units_sold"`
	Revenue   float64 `json:"revenue"`
}

type SalesSummary struct {
	TotalRevenue      float64 `json:"total_revenue"`
	TotalOrders       int64   `json:"total_orders"`
	TotalDiscount     float64 `json:"total_discount"`
	DeliveredOrders   int64   `json:"delivered_orders"`
	PendingOrders     int64   `json:"pending_orders"`
	CancelledOrders   int64   `json:"cancelled_orders"`
	CompletedOrders   int64   `json:"completed_orders"`
	CouponDiscount    float64 `json:"coupon_discount"`
	OfferDiscount     float64 `json:"offer_discount"`
}

func ExportSalesReport(c *gin.Context) {
	format := c.DefaultQuery("format", "excel")
	filter := c.DefaultQuery("filter", "monthly")
	// preview := c.DefaultQuery("preview", "false")
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	db := database.GetDB()
	
	// Determine date range
	var startDate, endDate time.Time
	var reportPeriod string
	
	if startDateStr != "" && endDateStr != "" {
		startDate, _ = time.Parse("2006-01-02", startDateStr)
		endDate, _ = time.Parse("2006-01-02", endDateStr)
		reportPeriod = fmt.Sprintf("%s to %s (custom)", startDateStr, endDateStr)
	} else {
		switch filter {
		case "daily":
			startDate = time.Now().AddDate(0, 0, -1)
			endDate = time.Now()
			reportPeriod = time.Now().Format("2006-01-02") + " (daily)"
		case "weekly":
			startDate = time.Now().AddDate(0, 0, -7)
			endDate = time.Now()
			reportPeriod = startDate.Format("2006-01-02") + " to " + endDate.Format("2006-01-02") + " (weekly)"
		case "monthly":
			startDate = time.Now().AddDate(0, -1, 0)
			endDate = time.Now()
			reportPeriod = startDate.Format("2006-01-02") + " to " + endDate.Format("2006-01-02") + " (monthly)"
		case "yearly":
			startDate = time.Now().AddDate(-1, 0, 0)
			endDate = time.Now()
			reportPeriod = startDate.Format("2006-01-02") + " to " + endDate.Format("2006-01-02") + " (yearly)"
		default:
			startDate = time.Now().AddDate(0, -1, 0)
			endDate = time.Now()
			reportPeriod = startDate.Format("2006-01-02") + " to " + endDate.Format("2006-01-02") + " (monthly)"
		}
	}

	// Get comprehensive summary data with all order status counts
	var summary SalesSummary
	
	// Get total orders count
	db.Raw(`
		SELECT COUNT(*) as total_orders 
		FROM orders 
		WHERE created_at >= ? AND created_at <= ? 
		AND status NOT IN ('cancelled', 'refunded')
	`, startDate, endDate).Scan(&summary.TotalOrders)

	// Get revenue and discount data
	db.Raw(`
		SELECT 
			COALESCE(SUM(total_price), 0) as total_revenue,
			COALESCE(SUM(total_discount), 0) as total_discount,
			COALESCE(SUM(coupon_discount), 0) as coupon_discount,
			COALESCE(SUM(total_discount - coupon_discount), 0) as offer_discount
		FROM orders 
		WHERE created_at >= ? AND created_at <= ? 
		AND status NOT IN ('cancelled', 'refunded')
		AND (payment_status = 'Paid' OR (payment_method = 'COD' AND status = 'Delivered'))
	`, startDate, endDate).Scan(&summary)

	// Get order status counts
	db.Raw(`
		SELECT 
			COUNT(CASE WHEN status = 'Delivered' THEN 1 END) as delivered_orders,
			COUNT(CASE WHEN status IN ('Pending', 'Confirmed', 'Shipped', 'Out for Delivery') THEN 1 END) as pending_orders,
			COUNT(CASE WHEN status IN ('Return Rejected', 'Cancelled', 'Returned') THEN 1 END) as cancelled_orders,
			COUNT(CASE WHEN status = 'Delivered' THEN 1 END) as completed_orders
		FROM orders 
		WHERE created_at >= ? AND created_at <= ? 
		AND status NOT IN ('cancelled', 'refunded')
	`, startDate, endDate).Scan(&summary)

	// Calculate average order value
	avgOrderValue := 0.0
	if summary.DeliveredOrders > 0 {
		avgOrderValue = summary.TotalRevenue / float64(summary.DeliveredOrders)
	}

	// Get order details
	var orderDetails []OrderDetail
	db.Raw(`
		SELECT 
			o.order_id_unique as order_id,
			u.user_name as customer,
			DATE(o.created_at) as date,
			o.total_price as amount,
			o.total_discount as discount,
			o.status
		FROM orders o
		LEFT JOIN users u ON o.user_id = u.id
		WHERE o.created_at >= ? AND o.created_at <= ? 
			AND o.status NOT IN ('cancelled', 'refunded')
		ORDER BY o.created_at DESC
	`, startDate, endDate).Scan(&orderDetails)

	// Get top selling products
	var topProducts []ProductSales
	db.Raw(`
		SELECT 
			p.product_name,
			SUM(oi.quantity) as units_sold,
			SUM(oi.item_total) as revenue
		FROM order_items oi
		JOIN products p ON oi.product_id = p.id
		JOIN orders o ON oi.order_id = o.id
		WHERE o.created_at >= ? AND o.created_at <= ? 
			AND o.status NOT IN ('cancelled', 'refunded')
		GROUP BY p.product_name
		ORDER BY units_sold DESC, revenue DESC
		LIMIT 10
	`, startDate, endDate).Scan(&topProducts)

	// Get top selling categories
	var topCategories []CategorySales
	db.Raw(`
		SELECT 
			c.category_name,
			SUM(oi.quantity) as units_sold,
			SUM(oi.item_total) as revenue
		FROM order_items oi
		JOIN products p ON oi.product_id = p.id
		JOIN categories c ON p.category_id = c.id
		JOIN orders o ON oi.order_id = o.id
		WHERE o.created_at >= ? AND o.created_at <= ? 
			AND o.status NOT IN ('cancelled', 'refunded')
		GROUP BY c.category_name
		ORDER BY units_sold DESC, revenue DESC
		LIMIT 10
	`, startDate, endDate).Scan(&topCategories)

	// Get top selling brands - FIXED QUERY
	var topBrands []BrandSales
	db.Raw(`
		SELECT 
			COALESCE(NULLIF(p.brand, ''), 'Unknown Brand') AS brand_name,
			SUM(oi.quantity) as units_sold,
			SUM(oi.item_total) as revenue
		FROM order_items oi
		JOIN products p ON oi.product_id = p.id
		JOIN orders o ON oi.order_id = o.id
		WHERE o.created_at >= ? AND o.created_at <= ? 
			AND o.status NOT IN ('cancelled', 'refunded')
			AND p.brand IS NOT NULL 
			AND p.brand != ''
		GROUP BY p.brand
		HAVING SUM(oi.quantity) > 0
		ORDER BY units_sold DESC, revenue DESC
		LIMIT 10
	`, startDate, endDate).Scan(&topBrands)

	// If no brands found, add a placeholder
	if len(topBrands) == 0 {
		topBrands = []BrandSales{
			{BrandName: "No Brands Sold", UnitsSold: 0, Revenue: 0},
		}
	}

	switch format {
	case "excel":
		f := excelize.NewFile()
		
		// Set document properties
		f.SetDocProps(&excelize.DocProperties{
			Title:       "Silver Sales Report",
			Subject:     "Sales Report",
			Creator:     "Silver E-commerce",
			Description: "Sales report generated by Silver E-commerce System",
		})

		// Main Report Sheet
		f.SetSheetName("Sheet1", "Sales Report")
		
		// Title
		f.SetCellValue("Sales Report", "A1", "Silver Sales Report")
		f.MergeCell("Sales Report", "A1", "H1")
		styleTitle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 18, Color: "1F4E78"},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		f.SetCellStyle("Sales Report", "A1", "H1", styleTitle)

		// Report Period
		f.SetCellValue("Sales Report", "A2", "Report Period: "+reportPeriod)
		f.MergeCell("Sales Report", "A2", "H2")
		stylePeriod, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Size: 12, Color: "2F5496"},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		f.SetCellStyle("Sales Report", "A2", "H2", stylePeriod)

		// Summary Section
		f.SetCellValue("Sales Report", "A4", "Summary")
		styleSummaryTitle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 14, Color: "1F4E78"},
		})
		f.SetCellStyle("Sales Report", "A4", "A4", styleSummaryTitle)

		// Enhanced summary data with all order status counts and discount breakdown
		summaryData := [][]interface{}{
			{"Total Revenue", fmt.Sprintf("$%.2f", summary.TotalRevenue)},
			{"Total Orders", summary.TotalOrders},
			{"Completed Orders", summary.CompletedOrders},
			{"Pending Orders", summary.PendingOrders},
			{"Cancelled Orders", summary.CancelledOrders},
			{"Total Discount", fmt.Sprintf("$%.2f", summary.TotalDiscount)},
			{"Total Amount", fmt.Sprintf("$%.2f", summary.TotalRevenue)},
		}

		for i, data := range summaryData {
			f.SetCellValue("Sales Report", fmt.Sprintf("A%d", i+5), data[0])
			f.SetCellValue("Sales Report", fmt.Sprintf("B%d", i+5), data[1])
		}

		// Order Details Section
		orderStartRow := len(summaryData) + 8
		f.SetCellValue("Sales Report", fmt.Sprintf("A%d", orderStartRow), "Order Details")
		f.SetCellStyle("Sales Report", fmt.Sprintf("A%d", orderStartRow), fmt.Sprintf("A%d", orderStartRow), styleSummaryTitle)

		// Order Details Headers
		orderHeaders := []string{"Order ID", "Customer", "Date", "Amount", "Discount", "Status"}
		for i, header := range orderHeaders {
			f.SetCellValue("Sales Report", fmt.Sprintf("%c%d", 'A'+i, orderStartRow+1), header)
		}
		styleHeader, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"2F5496"}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		f.SetCellStyle("Sales Report", fmt.Sprintf("A%d", orderStartRow+1), fmt.Sprintf("F%d", orderStartRow+1), styleHeader)

		// Order Details Data
		for i, order := range orderDetails {
			row := orderStartRow + 2 + i
			f.SetCellValue("Sales Report", fmt.Sprintf("A%d", row), order.OrderID)
			f.SetCellValue("Sales Report", fmt.Sprintf("B%d", row), order.Customer)
			f.SetCellValue("Sales Report", fmt.Sprintf("C%d", row), order.Date)
			f.SetCellValue("Sales Report", fmt.Sprintf("D%d", row), fmt.Sprintf("$%.2f", order.Amount))
			f.SetCellValue("Sales Report", fmt.Sprintf("E%d", row), fmt.Sprintf("$%.2f", order.Discount))
			f.SetCellValue("Sales Report", fmt.Sprintf("F%d", row), order.Status)
		}

		// Top Selling Products Section
		productsStartRow := orderStartRow + len(orderDetails) + 4
		f.SetCellValue("Sales Report", fmt.Sprintf("A%d", productsStartRow), "Top 10 Best-Selling Products")
		f.SetCellStyle("Sales Report", fmt.Sprintf("A%d", productsStartRow), fmt.Sprintf("A%d", productsStartRow), styleSummaryTitle)

		productHeaders := []string{"Product Name", "Units Sold", "Revenue"}
		for i, header := range productHeaders {
			f.SetCellValue("Sales Report", fmt.Sprintf("%c%d", 'A'+i, productsStartRow+1), header)
		}
		f.SetCellStyle("Sales Report", fmt.Sprintf("A%d", productsStartRow+1), fmt.Sprintf("C%d", productsStartRow+1), styleHeader)

		for i, product := range topProducts {
			row := productsStartRow + 2 + i
			f.SetCellValue("Sales Report", fmt.Sprintf("A%d", row), product.ProductName)
			f.SetCellValue("Sales Report", fmt.Sprintf("B%d", row), product.UnitsSold)
			f.SetCellValue("Sales Report", fmt.Sprintf("C%d", row), fmt.Sprintf("$%.2f", product.Revenue))
		}

		// Top Selling Categories Section
		categoriesStartRow := productsStartRow + len(topProducts) + 4
		f.SetCellValue("Sales Report", fmt.Sprintf("A%d", categoriesStartRow), "Top 10 Best-Selling Categories")
		f.SetCellStyle("Sales Report", fmt.Sprintf("A%d", categoriesStartRow), fmt.Sprintf("A%d", categoriesStartRow), styleSummaryTitle)

		categoryHeaders := []string{"Category Name", "Units Sold", "Revenue"}
		for i, header := range categoryHeaders {
			f.SetCellValue("Sales Report", fmt.Sprintf("%c%d", 'A'+i, categoriesStartRow+1), header)
		}
		f.SetCellStyle("Sales Report", fmt.Sprintf("A%d", categoriesStartRow+1), fmt.Sprintf("C%d", categoriesStartRow+1), styleHeader)

		for i, category := range topCategories {
			row := categoriesStartRow + 2 + i
			f.SetCellValue("Sales Report", fmt.Sprintf("A%d", row), category.CategoryName)
			f.SetCellValue("Sales Report", fmt.Sprintf("B%d", row), category.UnitsSold)
			f.SetCellValue("Sales Report", fmt.Sprintf("C%d", row), fmt.Sprintf("$%.2f", category.Revenue))
		}

		// Top Selling Brands Section
		brandsStartRow := categoriesStartRow + len(topCategories) + 4
		f.SetCellValue("Sales Report", fmt.Sprintf("A%d", brandsStartRow), "Top 10 Best-Selling Brands")
		f.SetCellStyle("Sales Report", fmt.Sprintf("A%d", brandsStartRow), fmt.Sprintf("A%d", brandsStartRow), styleSummaryTitle)

		brandHeaders := []string{"Brand Name", "Units Sold", "Revenue"}
		for i, header := range brandHeaders {
			f.SetCellValue("Sales Report", fmt.Sprintf("%c%d", 'A'+i, brandsStartRow+1), header)
		}
		f.SetCellStyle("Sales Report", fmt.Sprintf("A%d", brandsStartRow+1), fmt.Sprintf("C%d", brandsStartRow+1), styleHeader)

		for i, brand := range topBrands {
			row := brandsStartRow + 2 + i
			f.SetCellValue("Sales Report", fmt.Sprintf("A%d", row), brand.BrandName)
			f.SetCellValue("Sales Report", fmt.Sprintf("B%d", row), brand.UnitsSold)
			f.SetCellValue("Sales Report", fmt.Sprintf("C%d", row), fmt.Sprintf("$%.2f", brand.Revenue))
		}

		// Auto-size columns
		f.SetColWidth("Sales Report", "A", "H", 15)
		f.SetColWidth("Sales Report", "A", "A", 25) // Make first column wider for titles

		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", "attachment; filename=silver_sales_report_"+time.Now().Format("20060102")+".xlsx")
		
		if err := f.Write(c.Writer); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to generate Excel file"})
			return
		}

	case "pdf":
		cfg := config.NewBuilder().Build()
		m := maroto.New(cfg)

		// Title
		m.AddRows(
			text.NewRow(20, "Silver Sales Report", props.Text{
				Top:   2,
				Size:  16,
				Style: fontstyle.Bold,
				Align: align.Center,
				Color: &props.Color{Red: 31, Green: 78, Blue: 120},
			}),
			text.NewRow(10, "Report Period: "+reportPeriod, props.Text{
				Size:  10,
				Align: align.Center,
				Color: &props.Color{Red: 47, Green: 84, Blue: 150},
			}),
		)

		// Enhanced Summary Section with all order status counts
		m.AddRows(
			text.NewRow(12, "Summary", props.Text{
				Size:  12,
				Style: fontstyle.Bold,
				Color: &props.Color{Red: 31, Green: 78, Blue: 120},
			}),
		)

		summaryRows := [][]string{
			{"Total Revenue", fmt.Sprintf("$%.2f", summary.TotalRevenue)},
			{"Total Orders", strconv.FormatInt(summary.TotalOrders, 10)},
			{"Completed Orders", strconv.FormatInt(summary.CompletedOrders, 10)},
			{"Pending Orders", strconv.FormatInt(summary.PendingOrders, 10)},
			{"Cancelled Orders", strconv.FormatInt(summary.CancelledOrders, 10)},
			{"Total Discount", fmt.Sprintf("$%.2f", summary.TotalDiscount)},
			{"Total Amount", fmt.Sprintf("$%.2f", summary.TotalRevenue)},
		}

		for _, rowData := range summaryRows {
			m.AddRows(
				row.New(8).Add(
					text.NewCol(6, rowData[0], props.Text{Size: 9}),
					text.NewCol(6, rowData[1], props.Text{Size: 9, Style: fontstyle.Bold}),
				),
			)
		}

		// Order Details Section
		m.AddRows(
			text.NewRow(8, ""),
			text.NewRow(12, "Order Details", props.Text{
				Size:  12,
				Style: fontstyle.Bold,
				Color: &props.Color{Red: 31, Green: 78, Blue: 120},
			}),
			row.New(10).Add(
				text.NewCol(3, "Order ID", props.Text{Size: 8, Style: fontstyle.Bold}),
				text.NewCol(3, "Customer", props.Text{Size: 8, Style: fontstyle.Bold}),
				text.NewCol(2, "Date", props.Text{Size: 8, Style: fontstyle.Bold}),
				text.NewCol(2, "Amount", props.Text{Size: 8, Style: fontstyle.Bold}),
				text.NewCol(2, "Status", props.Text{Size: 8, Style: fontstyle.Bold}),
			),
		)

		for _, order := range orderDetails {
			m.AddRows(
				row.New(8).Add(
					text.NewCol(3, order.OrderID, props.Text{Size: 7}),
					text.NewCol(3, order.Customer, props.Text{Size: 7}),
					text.NewCol(2, order.Date, props.Text{Size: 7}),
					text.NewCol(2, fmt.Sprintf("$%.2f", order.Amount), props.Text{Size: 7}),
					text.NewCol(2, order.Status, props.Text{Size: 7}),
				),
			)
		}

		// Top Products Section
		m.AddRows(
			text.NewRow(8, ""),
			text.NewRow(12, "Top 10 Best-Selling Products", props.Text{
				Size:  12,
				Style: fontstyle.Bold,
				Color: &props.Color{Red: 31, Green: 78, Blue: 120},
			}),
			row.New(10).Add(
				text.NewCol(8, "Product Name", props.Text{Size: 8, Style: fontstyle.Bold}),
				text.NewCol(2, "Units", props.Text{Size: 8, Style: fontstyle.Bold}),
				text.NewCol(2, "Revenue", props.Text{Size: 8, Style: fontstyle.Bold}),
			),
		)

		for _, product := range topProducts {
			m.AddRows(
				row.New(8).Add(
					text.NewCol(8, product.ProductName, props.Text{Size: 7}),
					text.NewCol(2, strconv.FormatInt(product.UnitsSold, 10), props.Text{Size: 7}),
					text.NewCol(2, fmt.Sprintf("$%.2f", product.Revenue), props.Text{Size: 7}),
				),
			)
		}

		// Top Categories Section
		m.AddRows(
			text.NewRow(8, ""),
			text.NewRow(12, "Top 10 Best-Selling Categories", props.Text{
				Size:  12,
				Style: fontstyle.Bold,
				Color: &props.Color{Red: 31, Green: 78, Blue: 120},
			}),
			row.New(10).Add(
				text.NewCol(8, "Category Name", props.Text{Size: 8, Style: fontstyle.Bold}),
				text.NewCol(2, "Units", props.Text{Size: 8, Style: fontstyle.Bold}),
				text.NewCol(2, "Revenue", props.Text{Size: 8, Style: fontstyle.Bold}),
			),
		)

		for _, category := range topCategories {
			m.AddRows(
				row.New(8).Add(
					text.NewCol(8, category.CategoryName, props.Text{Size: 7}),
					text.NewCol(2, strconv.FormatInt(category.UnitsSold, 10), props.Text{Size: 7}),
					text.NewCol(2, fmt.Sprintf("$%.2f", category.Revenue), props.Text{Size: 7}),
				),
			)
		}

		// Top Brands Section
		m.AddRows(
			text.NewRow(8, ""),
			text.NewRow(12, "Top 10 Best-Selling Brands", props.Text{
				Size:  12,
				Style: fontstyle.Bold,
				Color: &props.Color{Red: 31, Green: 78, Blue: 120},
			}),
			row.New(10).Add(
				text.NewCol(8, "Brand Name", props.Text{Size: 8, Style: fontstyle.Bold}),
				text.NewCol(2, "Units", props.Text{Size: 8, Style: fontstyle.Bold}),
				text.NewCol(2, "Revenue", props.Text{Size: 8, Style: fontstyle.Bold}),
			),
		)

		for _, brand := range topBrands {
			m.AddRows(
				row.New(8).Add(
					text.NewCol(8, brand.BrandName, props.Text{Size: 7}),
					text.NewCol(2, strconv.FormatInt(brand.UnitsSold, 10), props.Text{Size: 7}),
					text.NewCol(2, fmt.Sprintf("$%.2f", brand.Revenue), props.Text{Size: 7}),
				),
			)
		}

		   pdf, err := m.Generate()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to generate PDF"})
        return
    }

    c.Header("Content-Type", "application/pdf")
    // Always use inline for preview - user can manually download from browser
    c.Header("Content-Disposition", "inline; filename=silver_sales_report_"+time.Now().Format("20060102")+".pdf")
    c.Data(http.StatusOK, "application/pdf", pdf.GetBytes())
		c.Data(http.StatusOK, "application/pdf", pdf.GetBytes())

	default:
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"report": gin.H{
				"period": reportPeriod,
				"summary": gin.H{
					"total_revenue":        summary.TotalRevenue,
					"total_orders":         summary.TotalOrders,
					"delivered_orders":     summary.DeliveredOrders,
					"completed_orders":     summary.CompletedOrders,
					"pending_orders":       summary.PendingOrders,
					"cancelled_orders":     summary.CancelledOrders,
					"average_order_value":  avgOrderValue,
					"total_discount":       summary.TotalDiscount,
					"coupon_discount":      summary.CouponDiscount,
					"offer_discount":       summary.OfferDiscount,
					"total_amount":         summary.TotalRevenue,
				},
				"order_details":    orderDetails,
				"top_products":     topProducts,
				"top_categories":   topCategories,
				"top_brands":       topBrands,
			},
		})
	}
}