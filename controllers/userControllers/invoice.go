package controllers

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
)

func DownloadInvoice(c *gin.Context) {
	userID, _ := c.Get("id")
	orderID := c.Param("order_id")
	var order userModels.Orders
	if err := database.DB.Where("order_id_unique = ? AND user_id = ?", orderID, userID).
		Preload("OrderItems.Product").Preload("OrderItems.Variants").First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	var address adminModels.ShippingAddress
	if err := database.DB.Where("user_id = ?", userID).First(&address).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
	}
	user, _ := c.Get("user_name")

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
	pdf.Cell(95, 5, "Company Name : Silver")
	pdf.Cell(95, 5, fmt.Sprintf("Customer Name :%s", user))
	pdf.Ln(7)
	pdf.Cell(95, 5, "Street Address: Mumbai ")
	pdf.Cell(95, 5, fmt.Sprintf("Street Address: %s", address.City))
	pdf.Ln(15)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 7, fmt.Sprintf("Invoice Number: %s", order.OrderIdUnique))
	pdf.Ln(5)
	pdf.Cell(40, 7, fmt.Sprintf("Date: %s", order.OrderDate.Format("02/01/2006")))
	pdf.Ln(15)

	
	switch order.Status {
	case "Failed":
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

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 12)
	pdf.Ln(15)

	pdf.SetFont("Arial", "B", 12)
	header := []string{"Item", "Quantity", "Rate", "Amount"}
	colWidths := []float64{100, 30, 30, 30}

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

	totalAmount := 0.0
	for _, item := range order.OrderItems {
		pdf.Ln(5)
		itemName := fmt.Sprintf("%s (%s)", item.Product.ProductName, item.Variants.Color)
		quantity := fmt.Sprintf("%d", item.Quantity)
		rate := fmt.Sprintf("$%.2f", item.UnitPrice)
		amount := fmt.Sprintf("$%.2f", item.ItemTotal)

		pdf.CellFormat(colWidths[0], 10, itemName, "0", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[1], 10, quantity, "0", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[2], 10, rate, "0", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[3], 10, amount, "0", 0, "R", false, 0, "")
		pdf.Ln(10)
		totalAmount += item.ItemTotal
	}

	pageWidth, _ = pdf.GetPageSize()
	pdf.Line(pdf.GetX(), pdf.GetY(), pageWidth-20, pdf.GetY())
	pdf.Ln(5)

	pdf.SetFont("Arial", "B", 12)
	pageWidth, _ = pdf.GetPageSize()
	pdf.SetX(pageWidth - 60)
	pdf.Cell(25, 10, "Subtotal:")
	pdf.Cell(25, 10, fmt.Sprintf("$%.2f", order.Subtotal))
	pdf.Ln(10)

	if order.TotalDiscount > 0 {
		pageWidth, _ = pdf.GetPageSize()
		pdf.SetX(pageWidth - 60)
		pdf.Cell(25, 10, "Discount:")
		pdf.Cell(25, 10, fmt.Sprintf("-$%.2f", order.TotalDiscount))
		pdf.Ln(10)
	}
	pageWidth, _ = pdf.GetPageSize()
	pdf.SetX(pageWidth - 60)
	pdf.Cell(25, 10, "Shipping:")
	pdf.Cell(25, 10, "$10")
	pdf.Ln(10)
	pageWidth, _ = pdf.GetPageSize()
	pdf.SetX(pageWidth - 60)
	pdf.Cell(25, 10, "Total:")
	pdf.Cell(25, 10, fmt.Sprintf("$%.2f", order.TotalPrice))
	pdf.Ln(10)

	pageWidth, _ = pdf.GetPageSize()
	pdf.SetX(pageWidth - 60)
	pdf.SetFillColor(0, 0, 0)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(50, 15, fmt.Sprintf("Total $%.2f", order.TotalPrice), "", 0, "C", true, 0, "")

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=invoice_%s.pdf", order.OrderIdUnique))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PDF"})
		return
	}

	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}

