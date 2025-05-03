package controllers

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
	"github.com/razorpay/razorpay-go"
	"gorm.io/gorm"
)

var req struct {
	Reason string `json:"reason"`
}

func GetOrderList(c *gin.Context) {
	userID, _ := c.Get("id")
	var orders []userModels.Orders
	if err := database.DB.Where("user_id = ?", userID).
		Preload("OrderItems.Product").Order("created_at DESC").
		Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "order.html", gin.H{
			"status":        "success",
			"Orders":        orders,
			"UserName":      "Guest",
			"WishlistCount": 0,
			"CartCount":     0,
			"ProfileImage":  "",
		})
		return
	}

	userData := user.(userModels.Users)
	userNameStr := userName.(string)

	var wishlistCount, cartCount int64
	if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		cartCount = 0
	}

	c.HTML(http.StatusOK, "order.html", gin.H{
		"Orders":        orders,
		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
	})
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
		if err := tx.Where("user_id = ? AND order_id_unique = ?", userID, orderID).Preload("OrderItems").First(&order).Error; err != nil {
			return err
		}
		if order.Status != "Pending" && order.Status != "Confirmed" {
			return gin.Error{Meta: gin.H{"error": "Only pending or confirmed orders can be cancelled"}}
		}

		if order.Status == "Cancelled" {
			return gin.Error{Meta: gin.H{"error": "Order is already cancelled"}}
		}

		if order.PaymentMethod == "ONLINE" {
			var payment adminModels.PaymentDetails
			if err := tx.Where("order_id = ? AND status = ?", order.ID, "Success").First(&payment).Error; err != nil {
				return fmt.Errorf("payment details not found: %v", err)
			}

			wallet,err:=EnshureWallet(tx,uint(userID.(uint)))
			if err!=nil{
				return err
			}
			wallet.Balance+=payment.Amount
			if err:=tx.Save(&wallet).Error;err!=nil{
				return fmt.Errorf("failed to update wallet balance: %v", err)
			}

			payment.Status = "RefundedToWallet"
			if err:=tx.Save(&payment).Error;err!=nil{
				return fmt.Errorf("failed to update payment status: %v", err)
			}

		}

		for _, item := range order.OrderItems {
			if item.Status == "Active" {
				if err := incrementStock(tx, item.VariantsID, item.Quantity); err != nil {
					return err
				}
				item.Status = "Cancelled"
				if err := tx.Save(&item).Error; err != nil {
					return err
				}
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to cancel order: %v", err)})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Order cancelled successfully"})
}

func CancelOrderItem(c *gin.Context) {
	userID, _ := c.Get("id")
	orderID := c.Param("order_id")
	itemID := c.Param("item_id")

	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", "Please provide valid data", "")
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var order userModels.Orders
		if err := tx.Where("user_id = ? AND order_id_unique = ?", userID, orderID).Preload("OrderItems").First(&order).Error; err != nil {
			return err
		}

		if order.Status != "Pending" && order.Status != "Confirmed" {
			return gin.Error{Meta: gin.H{"error": "Only pending or confirmed orders can be modified"}}
		}

		if order.Status == "Cancelled" {
			return gin.Error{Meta: gin.H{"error": "Order is already cancelled"}}
		}

		itemIDUint, err := strconv.ParseUint(itemID, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid item ID: %v", err)
		}

		for i, item := range order.OrderItems {
			if item.ID == uint(itemIDUint) {
				if item.Status != "Active" {
					return gin.Error{Meta: gin.H{"error": "Item is already cancelled or not active"}}
				}

				if order.PaymentMethod == "ONLINE" {
					var payment adminModels.PaymentDetails
					if err := tx.Where("order_id = ? AND status = ?", order.ID, "Success").First(&payment).Error; err != nil {
						return fmt.Errorf("payment details not found: %v", err)
					}

					refundAmount:=item.Price * float64(item.Quantity)

					wallet,err:=EnshureWallet(tx,uint(userID.(uint)))
					if err!=nil{
						return err
					}

					wallet.Balance += refundAmount
					if err:=tx.Save(&wallet).Error;err!=nil{
						return fmt.Errorf("failed to update wallet balance: %v", err)
					}

					payment.Amount -= refundAmount	
					if payment.Amount <=0{
						payment.Status = "RefundedToWallet"
					}else{
						payment.Status = "PartiallyRefundedToWallet"
					}
					if err := tx.Save(&payment).Error; err != nil {
						return fmt.Errorf("failed to update payment amount: %v", err)
					}
				}

				if err := incrementStock(tx, item.VariantsID, item.Quantity); err != nil {
					return err
				}
				order.OrderItems[i].Status = "Cancelled"
				if err := tx.Save(&order.OrderItems[i]).Error; err != nil {
					return err
				}
				cancellation := userModels.Cancellation{OrderID: order.ID, ItemID: &order.OrderItems[i].ID, Reason: req.Reason}
				if err := tx.Create(&cancellation).Error; err != nil {
					return err
				}

				allCancelled := true
				for _, item := range order.OrderItems {
					if item.Status != "Cancelled" {
						allCancelled = false
						break
					}
				}

				if allCancelled {
					if order.PaymentMethod == "ONLINE" {
						var payment adminModels.PaymentDetails
						if err := tx.Where("order_id = ? AND status = ?", order.ID, "Success").First(&payment).Error; err != nil {
							return fmt.Errorf("payment details not found: %v", err)
						}
						if payment.Amount > 0 {
							client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))
							data := map[string]interface{}{
								"payment_id": payment.RazorpayPaymentID,
								"amount":     int(payment.Amount * 100),
								"speed":      "normal",
							}
							options := map[string]string{}
							_, err := client.Refund.Create(data, options)
							if err != nil {
								return fmt.Errorf("failed to initiate final refund: %v", err)
							}
							payment.Status = "Refunded"
							payment.Amount = 0
							if err := tx.Save(&payment).Error; err != nil {
								return fmt.Errorf("failed to update payment status: %v", err)
							}
						}
					}

					order.Status = "Cancelled"
					if err := tx.Save(&order).Error; err != nil {
						return err
					}

					orderCancellation := userModels.Cancellation{OrderID: order.ID, Reason: req.Reason}
					if err := tx.Create(&orderCancellation).Error; err != nil {
						return err
					}
				}

				return nil
			}
		}
		return gin.Error{Meta: gin.H{"error": "Item not found"}}
	})
	if err != nil {
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to cancel item: %v", err)})
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
		if err := tx.Where("order_id_unique = ? AND user_id = ?", orderID, userID).
			Preload("OrderItems").First(&order).Error; err != nil {
			return err
		}
		if order.Status != "Delivered" {
			return gin.Error{Meta: gin.H{"error": "Only delivered orders can be returned"}}
		}

		order.Status = "Return Requested"
		if err := tx.Save(&order).Error; err != nil {
			return err
		}

		returnReq := userModels.Return{OrderID: order.ID, Reason: req.Reason, Status: "Requested"}
		return tx.Create(&returnReq).Error
	})
	if err != nil {
		if ginErr, ok := err.(gin.Error); ok && ginErr.Meta != nil {
			c.JSON(http.StatusBadRequest, ginErr.Meta)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to request return: %v", err)})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Return requested successfully"})
}

func ShowOrderDetails(c *gin.Context) {
	userID, _ := c.Get("id")
	orderID := c.Param("order_id")
	var order userModels.Orders
	if err := database.DB.Where("order_id_unique = ? AND user_id = ?", orderID, userID).
		Preload("OrderItems.Product").Preload("OrderItems.Variants").
		First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	var address adminModels.ShippingAddress
	database.DB.Where("user_id = ? AND order_id = ?", userID, orderID).First(&address)

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "orderDetail.html", gin.H{
			"status":          "success",
			"Order":           order,
			"ShippingAddress": address,
			"UserName":        "Guest",
			"WishlistCount":   0,
			"CartCount":       0,
			"ProfileImage":    "",
		})
		return
	}

	userData := user.(userModels.Users)
	userNameStr := userName.(string)

	var wishlistCount, cartCount int64
	if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		cartCount = 0
	}
	c.HTML(http.StatusOK, "orderDetail.html", gin.H{
		"Order":           order,
		"ShippingAddress": address,
		"UserName":        userNameStr,
		"ProfileImage":    userData.ProfileImage,
		"WishlistCount":   wishlistCount,
		"CartCount":       cartCount,
	})
}

func DownloadInvoice(c *gin.Context) {
	userID, _ := c.Get("id")
	orderID := c.Param("order_id")
	var order userModels.Orders
	if err := database.DB.Where("order_id_unique = ? AND user_id = ?", orderID, userID).
		Preload("OrderItems.Product").Preload("OrderItems.Variants").First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, fmt.Sprintf("Invoice - %s", order.OrderIdUnique))
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

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=invoice_%s.pdf", order.OrderIdUnique))
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}

func SearchOrders(c *gin.Context) {
	userID, _ := c.Get("id")
	query := c.Query("q")
	var orders []userModels.Orders

	if err := database.DB.Where("user_id = ?", userID).
		Preload("OrderItems.Product").
		Where("order_id_unique LIKE ? OR status LIKE ?", "%"+query+"%", "%"+query+"%").
		Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search orders"})
		return
	}

	filteredOrders := []userModels.Orders{}
	for _, order := range orders {
		for _, item := range order.OrderItems {
			if strings.Contains(strings.ToLower(item.Product.ProductName), strings.ToLower(query)) {
				filteredOrders = append(filteredOrders, order)
				break
			}
		}
	}

	c.JSON(http.StatusOK, filteredOrders)
}

func incrementStock(tx *gorm.DB, variantID, quantity uint) error {
	var variant adminModels.Variants
	if err := tx.First(&variant, variantID).Error; err != nil {
		return err
	}
	variant.Stock += quantity
	return tx.Save(&variant).Error
}
