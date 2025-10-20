package controllers

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sojil8/eCommerce-silver/config"
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func ListOrder(c *gin.Context) {
	// Parse query parameters with defaults
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}
	search := strings.TrimSpace(c.Query("search"))
	filterStatus := strings.TrimSpace(c.Query("status"))
	sort := c.DefaultQuery("sort", "order_date desc")

	// Validate sort parameter to prevent SQL injection
	allowedSorts := map[string]bool{
		"order_date desc":  true,
		"order_date asc":   true,
		"total_price desc": true,
		"total_price asc":  true,
	}
	if !allowedSorts[sort] {
		pkg.Log.Warn("Invalid sort parameter", zap.String("sort", sort))
		sort = "order_date desc"
	}

	offset := (page - 1) * limit

	// Build query
	query := database.DB.Preload("OrderItems.Product").Preload("OrderItems.Variants").Preload("User")
	if search != "" {
		query = query.Joins("JOIN users ON users.id = orders.user_id").
			Where("orders.order_id_unique ILIKE ? OR users.user_name ILIKE ? OR users.email ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if filterStatus != "" {
		query = query.Where("orders.status = ?", filterStatus)
	}
	// Remove the exclusion of "Failed" if you want to include all statuses, or explicitly include "Returned"
	// query = query.Where("orders.status <> ?", "Failed") // Removed to allow all statuses

	// Count total orders
	var total int64
	if err := query.Model(&userModels.Orders{}).Count(&total).Error; err != nil {
		pkg.Log.Error("Failed to count orders", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch orders", err.Error(), "")
		return
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	// Fetch orders
	var orders []userModels.Orders
	if err := query.Order(sort).Limit(limit).Offset(offset).Find(&orders).Error; err != nil {
		pkg.Log.Error("Failed to fetch orders", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch orders", err.Error(), "")
		return
	}

	// Render template
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
	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	validStatuses := []string{"Pending", "Shipped", "Out for Delivery", "Delivered", "Cancelled", "Returned"}
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
			return fmt.Errorf("order not found: %v", err)
		}

		// Prevent changing status of final states
		if order.Status == "Cancelled" || order.Status == "Returned" {
			return gin.Error{Meta: gin.H{"error": "Cannot change status of Cancelled or Returned orders"}}
		}
		if order.Status == "Delivered" && req.Status != "Return Requested" {
			return gin.Error{Meta: gin.H{"error": "Delivered orders can only transition to Return Requested"}}
		}

		// Prevent direct transition to Cancelled or Returned
		if req.Status == "Cancelled" || req.Status == "Returned" {
			return gin.Error{Meta: gin.H{"error": "Cannot directly set status to Cancelled or Returned"}}
		}

		// Update PaymentStatus for COD orders when status changes to Delivered
		if req.Status == "Delivered" && order.PaymentMethod == "COD" {
			order.PaymentStatus = "Paid"
		}

		// Restock items if cancelling
		if req.Status == "Cancelled" {
			for _, item := range order.OrderItems {
				if err := incrementStock(tx, item.VariantsID, item.Quantity); err != nil {
					return fmt.Errorf("failed to restock item: %v", err)
				}
			}
		}

		order.Status = req.Status
		return tx.Save(&order).Error
	})

	if err != nil {
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update status: %v", err)})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Status updated"})
}
func ViewOrdetailsAdmin(c *gin.Context) {
	orderID := c.Param("order_id")
	var order userModels.Orders
	if err := database.DB.Where("order_id_unique = ?", orderID).
		Preload("OrderItems.Product").
		Preload("OrderItems.Variants").
		Preload("User").
		First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	var orderBackUp userModels.OrderBackUp
	if err := database.DB.Where("order_id_unique = ?", orderID).
		First(&orderBackUp).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "", "", "")
		return
	}

	var address adminModels.ShippingAddress
	if err := database.DB.Where("order_id = ?", orderID).First(&address).Error; err != nil {
		// Handle case where address might not exist
		address = adminModels.ShippingAddress{}
	}

	// Calculate current totals for active items
	var currentSubtotal float64
	var currentOfferDiscount float64
	var allItemsCancelled bool = true

	for _, item := range order.OrderItems {
		if item.Status != "Cancelled" {
			allItemsCancelled = false
			// itemTotal := (item.Product.Price + item.Variants.ExtraPrice - item.DiscountAmount) * float64(item.Quantity)
			currentSubtotal += (item.Product.Price + item.Variants.ExtraPrice) * float64(item.Quantity)
			currentOfferDiscount += item.DiscountAmount * float64(item.Quantity)
		}
	}

	// Calculate current total
	currentTotal := currentSubtotal - currentOfferDiscount - order.CouponDiscount + order.ShippingCost
	if allItemsCancelled {
		currentTotal = 0 // No active items, total is 0
	}

	// Update order in database if necessary
	if !allItemsCancelled && order.Status != "Cancelled" {
		order.Subtotal = currentSubtotal
		order.OfferDiscount = currentOfferDiscount
		order.TotalDiscount = currentOfferDiscount + order.CouponDiscount
		order.TotalPrice = currentTotal
		if err := database.DB.Save(&order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order totals"})
			return
		}
	} else if allItemsCancelled && order.Status != "Cancelled" {
		order.Status = "Cancelled"
		order.TotalPrice = 0
		order.Subtotal = 0
		order.TotalDiscount = 0
		if err := database.DB.Save(&order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
			return
		}
	}

	c.HTML(http.StatusOK, "orderDetailsAdmin.html", gin.H{
		"Order":                order,
		"Address":              address,
		"OrderBackUp":          orderBackUp,
		"CurrentSubtotal":      currentSubtotal,
		"CurrentTotal":         currentTotal,
		"CurrentOfferDiscount": currentOfferDiscount,
		"AllItemsCancelled":    allItemsCancelled,
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
			wallet, err := config.EnshureWallet(tx, returnReq.Order.UserID)
			if err != nil {
				return fmt.Errorf("failed to ensure wallet: %v", err)
			}
			currentBalance := wallet.Balance
			wallet.Balance += returnReq.Order.TotalPrice
			if err := tx.Save(&wallet).Error; err != nil {
				return fmt.Errorf("failed to update wallet balance: %v", err)
			}

			walletTransaction := userModels.WalletTransaction{
				UserID:        returnReq.Order.UserID,
				WalletID:      wallet.ID,
				Amount:        returnReq.Order.TotalPrice,
				LastBalance:   currentBalance,
				Description:   fmt.Sprintf("Refund for approved return of order %s", returnReq.Order.OrderIdUnique),
				Type:          "Credited",
				Receipt:       "rcpt-" + uuid.New().String(),
				OrderID:       returnReq.Order.OrderIdUnique,
				TransactionID: fmt.Sprintf("TXN-%d-%d", time.Now().UnixNano(), rand.Intn(10000)),
				PaymentMethod: returnReq.Order.PaymentMethod,
			}
			if err := tx.Create(&walletTransaction).Error; err != nil {
				pkg.Log.Error("Failed to create wallet transaction for return refund", zap.Uint("userID", returnReq.Order.UserID), zap.Error(err))
				return fmt.Errorf("failed to create wallet transaction: %v", err)
			}

			var payment adminModels.PaymentDetails
			if err := tx.Where("order_id = ? AND status IN ?", returnReq.Order.ID, []string{"Success", "PartiallyRefundedToWallet"}).
				First(&payment).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					pkg.Log.Warn("No payment details found for order_id", zap.Uint("orderID", returnReq.Order.ID))
				} else {
					return fmt.Errorf("failed to fetch payment details: %v", err)
				}
			} else {
				payment.Status = "RefundedToWallet"
				payment.Amount = 0
				if err := tx.Save(&payment).Error; err != nil {
					return fmt.Errorf("failed to update payment status: %v", err)
				}
			}

			// Update order PaymentStatus
			returnReq.Order.PaymentStatus = "RefundedToWallet"
			returnReq.Order.Status = "Returned"
			for _, item := range returnReq.Order.OrderItems {
				if item.Status == "Active" || item.Status == "Delivered" {
					if err := incrementStock(tx, item.VariantsID, item.Quantity); err != nil {
						return fmt.Errorf("failed to restock item: %v", err)
					}
					item.Status = "Returned"
					if err := tx.Save(&item).Error; err != nil {
						return fmt.Errorf("failed to update item status: %v", err)
					}
					if err := helper.UpdateProductStock(tx, item.ProductID); err != nil {
						return err
					}
				}
			}
		} else {
			for _, item := range returnReq.Order.OrderItems {
				if item.Status == "Active" || item.Status == "Delivered" {
					item.Status = "Return Rejected"
					if err := tx.Save(&item).Error; err != nil {
						return fmt.Errorf("failed to update item status: %v", err)
					}
				}
			}
			returnReq.Order.Status = "Return Rejected"
			returnReq.Order.PaymentStatus = "Paid" // Assuming the payment remains successful if return is rejected
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
