package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/config"
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ListOrder(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	sort := c.DefaultQuery("sort", "order_date desc")
	filterStatus := c.Query("status")

	offset := (page - 1) * limit
	var orders []userModels.Orders
	query := database.DB.Preload("OrderItems.Product").Preload("OrderItems.Variants").Preload("User")

	if search != "" {
		query = query.Where("order_id LIKE ? OR users.user_name LIKE ? OR users.email LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if filterStatus != "" {
		query = query.Where("status = ?", filterStatus)
	}

	var total int64
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	query.Model(&userModels.Orders{}).Count(&total)
	query.Order(sort).Limit(limit).Offset(offset).Find(&orders)

	c.HTML(http.StatusOK, "adminOrder.html", gin.H{
		"Orders":     orders,
		"Page":       page,
		"Limit":      limit,
		"Total":      total,
		"Search":     search,
		"Sort":       sort,
		"Filter":     filterStatus,
		"TotalPages": totalPages,
	})
}

var req struct {
	Status string `json:"status"`
}

func UpdateOrderStatus(c *gin.Context) {
	orderID := c.Param("order_id")

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	validStatuses := []string{"Pending", "Shipped", "Out for Delivery", "Delivered", "Cancelled"}
	isValid := false
	for _, s := range validStatuses {
		if s == req.Status {
			isValid = true
			break
		}
	}
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var order userModels.Orders
		if err := tx.Where("order_id_unique = ?", orderID).Preload("OrderItems").First(&order).Error; err != nil {
			return err
		}
		if order.Status == "Cancelled" || order.Status == "Delivered" {
			return gin.Error{Meta: gin.H{"error": "Cannot change status of Cancelled or Delivered orders"}}
		}
		if req.Status == "Cancelled" || req.Status == "Pending" {
			return gin.Error{Meta: gin.H{"error": "Only Pending orders can be cancelled"}}
		}

		if req.Status == "Cancelled" {
			for _, item := range order.OrderItems {
				if err := incrementStock(tx, item.VariantsID, item.Quantity); err != nil {
					return err
				}
			}
		}
		if req.Status == "Delivered"{
			order.Status = "Paid"
		}
		order.Status = req.Status
		return tx.Save(&order).Error
	})
	if err != nil {
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Status updated"})

}

func ViewOrdetailsAdmin(c *gin.Context) {
	orderID := c.Param("order_id")
	var order userModels.Orders
	if err := database.DB.Where("order_id_unique  = ?", orderID).Preload("OrderItems.Product").Preload("OrderItems.Variants").
		Preload("User").First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	var address adminModels.ShippingAddress
	database.DB.Where("order_id = ?", orderID).First(&address)
	c.HTML(http.StatusOK, "orderDetailsAdmin.html", gin.H{
		"Order":   order,
		"Address": address,
	})
}
func ListReturnRequests(c *gin.Context) {
	var returns []userModels.Return
	if err := database.DB.Preload("Order.OrderItems.Product").Preload("Order.User").Find(&returns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch return requests"})
		return
	}

	var ordersReturn userModels.Orders
	database.DB.Where("status = ?", "Returned").Find(&ordersReturn)

	c.HTML(http.StatusOK, "ReturnOrders.html", gin.H{
		"Returns": returns,
	})
}

type ReturnRequest struct {
	Approve bool `json:"approve"`
}

func VerifyReturnRequest(c *gin.Context) {
	returnID := c.Param("return_id")
	var returnRequest struct {
		Approve bool `json:"approve"`
	}

	if err := c.ShouldBindJSON(&returnRequest); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var returnReq userModels.Return
		if err := tx.Preload("Order.OrderItems").Preload("Order.User").
			First(&returnReq, returnID).Error; err != nil {
			return fmt.Errorf("return request not found: %v", err)
		}

		if returnReq.Order.Status != "Return Requested" {
			return gin.Error{Meta: gin.H{"error": "Return request already processed or invalid"}}
		}

		if returnRequest.Approve {
			// Handle wallet and payment for non-COD orders
			if returnReq.Order.PaymentMethod != "Cash On Delivery" {
				wallet, err := config.EnshureWallet(tx, returnReq.Order.UserID)
				if err != nil {
					return fmt.Errorf("failed to ensure wallet: %v", err)
				}
				wallet.Balance += returnReq.Order.TotalPrice
				if err := tx.Save(&wallet).Error; err != nil {
					return fmt.Errorf("failed to update wallet balance: %v", err)
				}

				var payment adminModels.PaymentDetails
				if err := tx.Where("order_id = ? AND status IN ?", returnReq.Order.ID, []string{"Success", "PartiallyRefundedToWallet"}).
					First(&payment).Error; err != nil {
					if err == gorm.ErrRecordNotFound {
						// Log warning but proceed if payment details are missing (e.g., COD or manual order)
						fmt.Printf("Warning: No payment details found for order_id=%d\n", returnReq.Order.ID)
					} else {
						return fmt.Errorf("failed to fetch payment details: %v", err)
					}
				} else {
					// Update payment status if payment record exists
					payment.Status = "RefundedToWallet"
					payment.Amount = 0
					if err := tx.Save(&payment).Error; err != nil {
						return fmt.Errorf("failed to update payment status: %v", err)
					}
				}
			}

			// Restock items
			for _, item := range returnReq.Order.OrderItems {
				if item.Status == "Active" || item.Status == "Delivered" {
					if err := incrementStock(tx, item.VariantsID, item.Quantity); err != nil {
						return fmt.Errorf("failed to restock item: %v", err)
					}
					item.Status = "Returned"
					if err := tx.Save(&item).Error; err != nil {
						return fmt.Errorf("failed to update item status: %v", err)
					}
				}
			}

			returnReq.Order.Status = "Returned"
		} else {
			// Reject return request
			for _, item := range returnReq.Order.OrderItems {
				if item.Status == "Active" || item.Status == "Delivered" {
					item.Status = "Return Rejected"
					if err := tx.Save(&item).Error; err != nil {
						return fmt.Errorf("failed to update item status: %v", err)
					}
				}
			}
			returnReq.Order.Status = "Return Rejected"
		}

		if err := tx.Save(&returnReq.Order).Error; err != nil {
			return fmt.Errorf("failed to update order status: %v", err)
		}

		if err := tx.Delete(&returnReq).Error; err != nil {
			return fmt.Errorf("failed to delete return request: %v", err)
		}

		return nil
	})

	if err != nil {
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to verify return: %v", err)})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Return request processed"})
}
func incrementStock(tx *gorm.DB, variantID, quantity uint) error {
	var variant adminModels.Variants
	if err := tx.First(&variant, variantID).Error; err != nil {
		return err
	}
	variant.Stock += quantity
	return tx.Save(&variant).Error
}
