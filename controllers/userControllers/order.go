package controllers

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
	"gorm.io/gorm"
)

func GetOrderList(c *gin.Context) {
	userID, _ := c.Get("id")
	var orders []userModels.Orders
	if err := database.DB.Where("user_id = ?", userID).Preload("OrderItems").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	c.HTML(http.StatusOK, "order.html", gin.H{
		"Orders":   orders,
		"UserName": c.GetString("user_name"),
	})

}

var req struct {
	Reason string `json:"reason"`
}

func CancelOrder(c *gin.Context) {
	userID, _ := c.Get("id")
	orderID := c.Param("order_id")
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", "Please provide valid data", "")
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var order userModels.Orders
		if err := tx.Where("user_id = ? AND order_id = ?", userID, orderID).Preload("OrderItems").First(&order).Error; err != nil {
			return err
		}

		if order.Status != "Pending" {
			return gin.Error{Meta: gin.H{"error": "Only pending orders can be cancelled"}}
		}

		for _, item := range order.OrderItems {
			if err := incrementStock(tx, item.VariantsID, item.Quantity); err != nil {
				return err
			}
			item.Status = "Cancelled"
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
		}
		order.Status = "Cancelled"
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		cancellation := userModels.Cancellation{OrderID: order.ID, Reason: req.Reason}
		return tx.Create(&cancellation).Error
	})
	if err != nil {
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Order cancelled successfully"})
}

func CancelOrderItem(c *gin.Context) {
	userId, _ := c.Get("id")
	orderID := c.Param("order_id")
	itemID := c.Param("item_id")

	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", "Please provide valid data", "")
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var order userModels.Orders
		if err := tx.Where("user_id = ? AND order_id = ?", userId, orderID).Preload("OrderItems").First(&order).Error; err != nil {
			return err
		}
		if order.Status != "Pending" {
			return gin.Error{Meta: gin.H{"error": "Only pending orders can be modified"}}
		}

		itemIDUint, _ := strconv.ParseUint(itemID, 10, 32)
		for i, item := range order.OrderItems {
			if item.ID == uint(itemIDUint) {
				if err := incrementStock(tx, item.VariantsID, item.Quantity); err != nil {
					return err
				}
				order.OrderItems[i].Status = "Cancelled"
				if err := tx.Save(&item).Error; err != nil {
					return err
				}
				cancellation := userModels.Cancellation{OrderID: order.ID, ItemID: &item.ID, Reason: req.Reason}
				return tx.Create(&cancellation).Error
			}
		}
		return gin.Error{Meta: gin.H{"error": "Item not found"}}
	})
	if err != nil {
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel item"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Item cancelled successfully"})
}

func ReturnOrder(c *gin.Context) {
	userID, _ := c.Get("id")
	orderID := c.Param("order_id")
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", "Reason is required", "")
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var order userModels.Orders
		if err := tx.Where("order_id = ? user_id = ?", orderID, userID).
			Preload("OrderItems").First(&order).Error; err != nil {
			return err
		}
		if order.Status != "Delivered" {
			return gin.Error{Meta: gin.H{"error": "Only delivered orders can be returned"}}
		}
		for _, item := range order.OrderItems {
			if item.Status == "Active" {
				if err := incrementStock(tx, item.VariantsID, item.Quantity); err != nil {
					return err
				}
				item.Status = "Returned"
				if err := tx.Save(&item).Error; err != nil {
					return err
				}
			}
		}
		order.Status = "Returned"
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		returnReq := userModels.Return{OrderID: order.ID, Reason: req.Reason}
		return tx.Create(&returnReq).Error
	})
	if err != nil {
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to return order"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Order returned successfully"})
}

func ShowOrderDetails(c *gin.Context) {
	userID, _ := c.Get("id")
	orderID := c.Param("order_id")
	fmt.Println(userID)
	var order userModels.Orders
	if err := database.DB.Where("order_id = ? AND user_id = ?", orderID, userID).
		Preload("OrderItems.Product").Preload("OrderItems.Variants").
		First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	var address userModels.Address
	database.DB.First(&address, order.AddressID)
	c.HTML(http.StatusOK, "orderDetail.html", gin.H{
		"Order":    order,
		"Address":  address,
		"UserName": c.GetString("user_name"),
	})
}
func DownloadInvoice(c *gin.Context) {
    userID, _ := c.Get("id")
    orderID := c.Param("order_id")
    var order userModels.Orders
    if err := database.DB.Where("order_id = ? AND user_id = ?", orderID, userID).
        Preload("OrderItems.Product").Preload("OrderItems.Variants").First(&order).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
        return
    }

    pdf := gofpdf.New("P", "mm", "A4", "")
    pdf.AddPage()
    pdf.SetFont("Arial", "B", 16)
    pdf.Cell(40, 10, fmt.Sprintf("Invoice - %s", order.OrderID))
    pdf.Ln(20)
    pdf.SetFont("Arial", "", 12)
    pdf.Cell(40, 10, fmt.Sprintf("Order Date: %s", order.OrderDate.Format("2006-01-02")))
    pdf.Ln(10)
    pdf.Cell(40, 10, fmt.Sprintf("Total: $%.2f", order.TotalPrice))
    pdf.Ln(10)
    pdf.Cell(40, 10, "Items:")
    pdf.Ln(10)
    for _, item := range order.OrderItems {
        pdf.Cell(40, 10, fmt.Sprintf("%s (%s) - Qty: %d - $%.2f", item.Product.ProductName, item.Variants.Color, item.Quantity, item.Price*float64(item.Quantity)))
        pdf.Ln(10)
    }

    var buf bytes.Buffer
    if err := pdf.Output(&buf); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PDF"})
        return
    }

    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=invoice_%s.pdf", order.OrderID))
    c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}
// ... (rest of the file remains unchanged)

func SearchOrders(c *gin.Context) {
	userID, _ := c.Get("id")
	query := c.Query("q")
	var orders []userModels.Orders
	if err := database.DB.Where("user_id = ? AND(order_id LIKE ? OR status LIKE ?)", userID, "%"+query+"%", "%"+query+"%").
		Preload("OrderItems").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search orders"})
		return
	}
	c.JSON(http.StatusOK, orders)
}

func incrementStock(tx *gorm.DB, variantID, quantity uint) error {
	var variant adminModels.Variants
	if err := tx.First(&variant, variantID).Error; err != nil {
		return err
	}
	variant.Stock += quantity
	return tx.Save(&variant).Error
}
