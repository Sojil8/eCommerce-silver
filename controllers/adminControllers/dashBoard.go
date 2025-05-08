package controllers

import (
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
)

func GetSalesReport(c *gin.Context) {
	// Get filter parameters
	reportType := c.DefaultQuery("type", "daily")
	startDateStr := c.DefaultQuery("start_date", "")
	endDateStr := c.DefaultQuery("end_date", "")

	var startDate, endDate time.Time
	var err error

	// Set default time period based on report type if custom dates not provided
	if startDateStr == "" || endDateStr == "" {
		now := time.Now()

		switch reportType {
		case "daily":
			startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			endDate = startDate.Add(24 * time.Hour)
		case "weekly":
			// Start from beginning of current week (assuming week starts on Monday)
			daysToSubtract := (int(now.Weekday()) + 6) % 7 // Convert Sunday=0 to Monday=0
			startDate = time.Date(now.Year(), now.Month(), now.Day()-daysToSubtract, 0, 0, 0, 0, now.Location())
			endDate = startDate.Add(7 * 24 * time.Hour)
		case "monthly":
			startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			endDate = startDate.AddDate(0, 1, 0)
		case "yearly":
			startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
			endDate = startDate.AddDate(1, 0, 0)
		default: // For custom date range
			if startDateStr != "" && endDateStr != "" {
				startDate, err = time.Parse("2006-01-02", startDateStr)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date format"})
					return
				}

				endDate, err = time.Parse("2006-01-02", endDateStr)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date format"})
					return
				}
				// Add a day to end date to include the full end date
				endDate = endDate.Add(24 * time.Hour)
			} else {
				// Default to today if nothing specified
				startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
				endDate = startDate.Add(24 * time.Hour)
			}
		}
	} else {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date format"})
			return
		}

		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date format"})
			return
		}
		// Add a day to end date to include the full end date
		endDate = endDate.Add(24 * time.Hour)
	}

	// Query for orders within the date range
	var orders []userModels.Orders
	if err := database.DB.
		Where("order_date BETWEEN ? AND ?", startDate, endDate).
		Where("status IN ?", []string{"Delivered", "Shipped", "Out for Delivery"}).
		Preload("OrderItems.Product").
		Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	// Calculate summary statistics
	var totalSales float64
	var totalDiscount float64
	var orderCount int

	for _, order := range orders {
		totalSales += order.TotalPrice
		totalDiscount += order.Discount
		orderCount++
	}

	// Calculate subtotal (total sales + discounts)
	subtotal := totalSales + totalDiscount

	// Format dates for display
	formattedStartDate := startDate.Format("Jan 02, 2006")
	formattedEndDate := endDate.Add(-24 * time.Hour).Format("Jan 02, 2006") // Subtract a day because we added one earlier

	c.HTML(http.StatusOK, "adminSalesReport.html", gin.H{
		"Orders":         orders,
		"TotalSales":     totalSales,
		"TotalDiscount":  totalDiscount,
		"Subtotal":       subtotal,
		"OrderCount":     orderCount,
		"ReportType":     reportType,
		"StartDate":      formattedStartDate,
		"EndDate":        formattedEndDate,
		"StartDateValue": startDate.Format("2006-01-02"),
		"EndDateValue":   endDate.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}
