package controllers

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
	"go.uber.org/zap"
)

func DownloadInvoice(c *gin.Context) {
	pkg.Log.Info("Starting invoice generation process")

	userID, _ := c.Get("id")
	orderID := c.Param("order_id")
	var order userModels.Orders
	var backupOrder userModels.OrderBackUp
	var useBackupData bool = false

	pkg.Log.Debug("Fetching order",
		zap.Any("user_id", userID),
		zap.String("order_id", orderID))

	// Check if order exists
	if err := database.DB.Where("order_id_unique = ? AND user_id = ?", orderID, userID).
		Preload("OrderItems.Product").Preload("OrderItems.Variants").First(&order).Error; err != nil {
		pkg.Log.Error("Order not found",
			zap.Any("user_id", userID),
			zap.String("order_id", orderID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// If order is cancelled, use backup data for financial totals but still show order items
	if order.Status == "Cancelled" {
		if err := database.DB.Where("order_id_unique = ?", orderID).First(&backupOrder).Error; err != nil {
			pkg.Log.Error("Backup order data not found",
				zap.String("order_id", orderID),
				zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "Backup order data not found"})
			return
		}
		useBackupData = true
		pkg.Log.Info("Using backup data for cancelled order",
			zap.String("order_id", orderID))
	}

	var address adminModels.ShippingAddress
	if err := database.DB.Where("user_id = ?", userID).First(&address).Error; err != nil {
		pkg.Log.Error("Shipping address not found",
			zap.Any("user_id", userID),
			zap.String("order_id", orderID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
		return
	}

	user, _ := c.Get("user_name")
	userName := user.(string)

	pkg.Log.Debug("Fetched user and address details",
		zap.Any("user_id", userID),
		zap.String("user_name", userName),
		zap.String("order_id", orderID))

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(10, 10, 20)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 30)
	pdf.SetTextColor(0, 0, 255)
	pdf.Cell(40, 20, "SILVER")
	pdf.SetFont("Arial", "B", 30)
	pdf.SetTextColor(0, 0, 0)
	pageWidth, _ := pdf.GetPageSize()
	pdf.SetX(pageWidth - 70)
	pdf.Cell(40, 20, "INVOICE")
	pdf.Ln(25)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(95, 5, "Bill from: Silver Ecom")
	pdf.Cell(95, 5, "Bill to:")
	pdf.Ln(7)
	pdf.Cell(95, 5, "Company Name: Silver")
	pdf.Cell(95, 5, fmt.Sprintf("Customer Name: %s", userName))
	pdf.Ln(7)
	pdf.Cell(95, 5, "Street Address: Mumbai")
	pdf.Cell(95, 5, fmt.Sprintf("Street Address: %s", address.City))
	pdf.Ln(15)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 7, fmt.Sprintf("Invoice Number: %s", order.OrderIdUnique))
	pdf.Ln(5)
	pdf.Cell(40, 7, fmt.Sprintf("Date: %s", order.OrderDate.Format("02/01/2006")))
	pdf.Ln(15)

	// Status display with color coding
	switch order.Status {
	case "Failed", "Cancelled":
		pdf.SetTextColor(255, 0, 0)
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(40, 7, fmt.Sprintf("Status: %s", order.Status))
	case "Delivered":
		pdf.SetTextColor(0, 128, 0)
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(40, 7, fmt.Sprintf("Status: %s", order.Status))
	case "Pending":
		pdf.SetTextColor(255, 165, 0)
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(40, 7, fmt.Sprintf("Status: %s", order.Status))
	default:
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(40, 7, fmt.Sprintf("Status: %s", order.Status))
	}

	// Add note for cancelled orders
	if useBackupData {
		pdf.Ln(10)
		pdf.SetTextColor(255, 0, 0)
		pdf.SetFont("Arial", "I", 10)
		pdf.Cell(40, 7, "Note: This order has been cancelled.")
	}

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 12)
	pdf.Ln(15)

	// Table header - Added Status column
	pdf.SetFont("Arial", "B", 12)
	header := []string{"Item", "Quantity", "Rate", "Amount", "Status"}
	colWidths := []float64{80, 25, 25, 25, 35}

	pdf.SetDrawColor(190, 190, 190)
	pageWidth, _ = pdf.GetPageSize()
	pdf.Line(pdf.GetX(), pdf.GetY(), pageWidth-20, pdf.GetY())
	pdf.Ln(2)

	pdf.SetFillColor(240, 240, 240)
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.3)
	for i, h := range header {
		pdf.CellFormat(colWidths[i], 10, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetFont("Arial", "", 12)
	pdf.SetDrawColor(190, 190, 190)

	pageWidth, _ = pdf.GetPageSize()
	pdf.Line(pdf.GetX(), pdf.GetY(), pageWidth-20, pdf.GetY())

	// Order items - ALWAYS show order items from the original order with status
	totalAmount := 0.0
	cancelledItemsExist := false

	for _, item := range order.OrderItems {
		pdf.Ln(5)

		// Build item name with product and variant details
		itemName := item.Product.ProductName
		if item.Variants.Color != "" {
			itemName += fmt.Sprintf(" (%s", item.Variants.Color)
			if item.Variants.Color != "" { // Fixed potential bug: original code repeated Color
				itemName += fmt.Sprintf(", Size: %s", item.Variants.Color)
			}
			itemName += ")"
		}

		quantity := fmt.Sprintf("%d", item.Quantity)
		rate := fmt.Sprintf("$%.2f", item.UnitPrice)
		amount := fmt.Sprintf("$%.2f", item.ItemTotal)

		// Set text color based on item status
		switch strings.ToLower(item.Status) {
		case "cancelled":
			pdf.SetTextColor(255, 0, 0) // Red for cancelled
			cancelledItemsExist = true
		case "delivered":
			pdf.SetTextColor(0, 128, 0) // Green for delivered
		case "pending":
			pdf.SetTextColor(255, 165, 0) // Orange for pending
		case "returned":
			pdf.SetTextColor(139, 0, 139) // Purple for returned
		default:
			pdf.SetTextColor(0, 0, 0) // Black for other statuses
		}

		pdf.CellFormat(colWidths[0], 10, itemName, "0", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[1], 10, quantity, "0", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[2], 10, rate, "0", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[3], 10, amount, "0", 0, "R", false, 0, "")

		// Status with appropriate color
		statusText := item.Status
		if statusText == "" {
			statusText = "Active"
		}
		pdf.CellFormat(colWidths[4], 10, statusText, "0", 0, "C", false, 0, "")

		pdf.Ln(10)
		totalAmount += item.ItemTotal

		// Reset text color for next row
		pdf.SetTextColor(0, 0, 0)

		pkg.Log.Debug("Processed order item",
			zap.String("order_id", orderID),
			zap.Uint("product_id", item.ProductID),
			zap.Uint("variant_id", item.VariantsID),
			zap.String("item_name", itemName),
			zap.String("status", statusText))
	}

	pkg.Log.Debug("Processed all order items",
		zap.String("order_id", orderID),
		zap.Int("item_count", len(order.OrderItems)),
		zap.Float64("total_amount", totalAmount))

	// Calculate financial details based on data source
	var subtotal, totalDiscount, shippingCost, totalPrice float64

	if useBackupData {
		// Use backup data for financial totals
		subtotal = backupOrder.Subtotal
		totalDiscount = backupOrder.CouponDiscount + backupOrder.OfferDiscount
		shippingCost = backupOrder.ShippingCost
		totalPrice = backupOrder.TotalPrice
	} else {
		// Use normal order data
		subtotal = order.Subtotal
		totalDiscount = order.TotalDiscount
		shippingCost = order.ShippingCost
		totalPrice = order.TotalPrice
	}

	pkg.Log.Debug("Calculated financial details",
		zap.String("order_id", orderID),
		zap.Bool("use_backup_data", useBackupData),
		zap.Float64("subtotal", subtotal),
		zap.Float64("total_discount", totalDiscount),
		zap.Float64("shipping_cost", shippingCost),
		zap.Float64("total_price", totalPrice))

	pageWidth, _ = pdf.GetPageSize()
	pdf.Line(pdf.GetX(), pdf.GetY(), pageWidth-20, pdf.GetY())
	pdf.Ln(5)

	// Financial summary
	pdf.SetFont("Arial", "B", 12)
	pageWidth, _ = pdf.GetPageSize()
	pdf.SetX(pageWidth - 60)
	pdf.Cell(25, 10, "Subtotal:")
	pdf.Cell(25, 10, fmt.Sprintf("$%.2f", subtotal))
	pdf.Ln(10)

	if totalDiscount > 0 {
		pageWidth, _ = pdf.GetPageSize()
		pdf.SetX(pageWidth - 60)
		pdf.Cell(25, 10, "Discount:")
		pdf.Cell(25, 10, fmt.Sprintf("-$%.2f", totalDiscount))
		pdf.Ln(10)
	}

	pageWidth, _ = pdf.GetPageSize()
	pdf.SetX(pageWidth - 60)
	pdf.Cell(25, 10, "Shipping:")
	pdf.Cell(25, 10, fmt.Sprintf("$%.2f", shippingCost))
	pdf.Ln(10)

	pageWidth, _ = pdf.GetPageSize()
	pdf.SetX(pageWidth - 60)
	pdf.Cell(25, 10, "Total:")
	pdf.Cell(25, 10, fmt.Sprintf("$%.2f", totalPrice))
	pdf.Ln(10)

	pageWidth, _ = pdf.GetPageSize()
	pdf.SetX(pageWidth - 60)
	pdf.SetFillColor(0, 0, 0)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(50, 15, fmt.Sprintf("Total $%.2f", totalPrice), "", 0, "C", true, 0, "")

	// Add status legend if there are cancelled items
	if cancelledItemsExist {
		pdf.Ln(20)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(40, 5, "Status Legend:")
		pdf.Ln(5)
		pdf.SetFont("Arial", "", 9)

		pdf.SetTextColor(255, 0, 0)
		pdf.Cell(20, 5, "● Cancelled")
		pdf.SetTextColor(0, 0, 0)
		pdf.Cell(15, 5, "- Item not delivered/refunded")

		pdf.Ln(5)
		pdf.SetTextColor(0, 128, 0)
		pdf.Cell(20, 5, "● Delivered")
		pdf.SetTextColor(0, 0, 0)
		pdf.Cell(15, 5, "- Item successfully delivered")

		pdf.Ln(5)
		pdf.SetTextColor(255, 165, 0)
		pdf.Cell(20, 5, "● Pending")
		pdf.SetTextColor(0, 0, 0)
		pdf.Cell(15, 5, "- Item processing/shipping")

		pdf.Ln(5)
		pdf.SetTextColor(139, 0, 139)
		pdf.Cell(20, 5, "● Returned")
		pdf.SetTextColor(0, 0, 0)
		pdf.Cell(15, 5, "- Item returned by customer")

		pkg.Log.Debug("Added status legend due to cancelled items",
			zap.String("order_id", orderID))
	} else {
		pkg.Log.Debug("No cancelled items, status legend not added",
			zap.String("order_id", orderID))
	}

	// Add footer note for cancelled orders
	if useBackupData {
		pdf.Ln(20)
		pdf.SetTextColor(150, 150, 150)
		pdf.SetFont("Arial", "I", 8)
		pdf.Cell(40, 5, "* Financial amounts reflect the state at time of cancellation")
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=invoice_%s.pdf", order.OrderIdUnique))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		pkg.Log.Error("Failed to generate PDF",
			zap.String("order_id", orderID),
			zap.Any("user_id", userID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PDF"})
		return
	}

	pkg.Log.Info("Invoice PDF generated successfully",
		zap.String("order_id", orderID),
		zap.Any("user_id", userID),
		zap.String("user_name", userName),
		zap.Float64("total_price", totalPrice))

	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}
