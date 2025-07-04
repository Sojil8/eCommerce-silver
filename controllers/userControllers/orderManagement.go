package controllers

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
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
		if err := tx.Where("user_id = ? AND order_id_unique = ?", userID, orderID).
			Preload("OrderItems").First(&order).Error; err != nil {
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
			if err := tx.Where("order_id = ? AND status IN ?", order.ID, []string{"Paid", "PartiallyRefundedToWallet"}).
				First(&payment).Error; err != nil {
				return fmt.Errorf("payment details not found: %v", err)
			}

			wallet, err := EnshureWallet(tx, uint(userID.(uint)))
			if err != nil {
				return err
			}

			refundAmount := order.TotalPrice
			wallet.Balance += refundAmount
			if err := tx.Save(&wallet).Error; err != nil {
				return fmt.Errorf("failed to update wallet balance: %v", err)
			}

			payment.Status = "RefundedToWallet"
			if err := tx.Save(&payment).Error; err != nil {
				return fmt.Errorf("failed to update payment status: %v", err)
			}

		}

		for _, item := range order.OrderItems {
			if item.Status == "Active" {
				var variant adminModels.Variants
				if err := tx.First(&variant, item.VariantsID).Error; err != nil {
					return err
				}
				variant.Stock += item.Quantity
				if err := tx.Save(&variant).Error; err != nil {
					return err
				}

				if err := helper.UpdateProductStock(tx, item.ProductID); err != nil {
					return err
				}

				item.Status = "Cancelled"
				if err := tx.Save(&item).Error; err != nil {
					return err
				}
			}
		}
		var coupons adminModels.Coupons
		if coupons.ID != 0 {
			if err := tx.First(&coupons, order.CouponID).Error; err != nil {
				return fmt.Errorf("failed to get the coupon detials: %v", err)
			} else {
				if !time.Now().Before(coupons.ExpiryDate) {
					coupons.UsedCount++

					if err := tx.Save(coupons).Error; err != nil {
						return fmt.Errorf("failed to update coupon used count: %v", err)
					}
				}
			}

		}

		order.PaymentStatus = "RefundedToWallet"
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
		if err := tx.Where("user_id = ? AND order_id_unique = ?", userID, orderID).Preload("OrderItems.Product").
			Preload("OrderItems.Variants").First(&order).Error; err != nil {
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

		var cancelItem *userModels.OrderItem
		for i, item := range order.OrderItems {
			if item.ID == uint(itemIDUint) {
				if item.Status != "Active" {
					return gin.Error{Meta: gin.H{"error": "Item is already cancelled or not active"}}
				}
				cancelItem = &order.OrderItems[i]
				break
			}
		}
		if cancelItem == nil {
			return gin.Error{Meta: gin.H{"error": "Item not found"}}
		}

		//bug area watch out........................................
		var variant adminModels.Variants
		if err := tx.First(&variant, cancelItem.VariantsID).Error; err != nil {
			return err
		}
		variant.Stock += cancelItem.Quantity
		if err := tx.Save(&variant).Error; err != nil {
			return err
		}

		remainingSubtotal := order.Subtotal - (cancelItem.UnitPrice+cancelItem.DiscountAmount/float64(cancelItem.Quantity))*float64(cancelItem.Quantity)
		log.Printf("CancelItem: ItemID=%d, Subtotal=%.2f, CancelItemTotal=%.2f, RemainingSubtotal=%.2f",
			cancelItem.ID, order.Subtotal, cancelItem.ItemTotal, remainingSubtotal)

		var coupon adminModels.Coupons
		couponDiscount := order.CouponDiscount
		var couponAdjustemnt float64

		if order.CouponID != 0 {
			if err := tx.First(&coupon, order.CouponID).Error; err != nil {
				if remainingSubtotal < coupon.MinPurchaseAmount {
					couponAdjustemnt = order.CouponDiscount
					order.CouponCode = ""
					order.CouponID = 0
					order.CouponID = 0
					log.Printf("Coupon %s removed: RemainingSubtotal=%.2f < MinPurchaseAmount=%.2f",
						coupon.CouponCode, remainingSubtotal, coupon.MinPurchaseAmount)
				} else {
					couponDiscount = remainingSubtotal * (coupon.DiscountPercentage / 100)
					couponAdjustemnt = order.CouponDiscount - couponDiscount
					log.Printf("Coupon %s adjusted: OldDiscount=%.2f, NewDiscount=%.2f",
						coupon.CouponCode, order.CouponDiscount, couponDiscount)
				}
			} else {
				log.Printf("Coupon ID %d not found: %v", order.CouponID, err)
				couponAdjustemnt = order.CouponDiscount
				couponDiscount = 0
				order.CouponID = 0
				order.CouponCode = ""
			}
		}

		refundAmount := cancelItem.ItemTotal
		refundAmount -= couponAdjustemnt
		if order.PaymentMethod == "ONLINE" {
			var payment adminModels.PaymentDetails
			if err := tx.Where("order_id = ? AND status IN ?", order.ID, []string{"Paid", "PartiallyRefundedToWallet"}).
				First(&payment).Error; err != nil {
				return fmt.Errorf("payment details not found: %v", err)
			}

			if refundAmount < 0 {
				log.Printf("Refund capped at 0: ItemTotal=%.2f, CouponAdjustment=%.2f", cancelItem.ItemTotal, couponAdjustemnt)
				refundAmount = 0
			}

			if refundAmount > 0 {
				wallet, err := EnshureWallet(tx, uint(userID.(uint)))
				if err != nil {
					return err
				}
				wallet.Balance += refundAmount
				if err := tx.Save(&wallet).Error; err != nil {
					return fmt.Errorf("failed to update wallet balance: %v", err)
				}

				payment.Amount -= refundAmount
				if payment.Amount <= 0 {
					payment.Status = "RefundedToWallet"
				} else {
					payment.Status = "PartiallyRefundedToWallet"
				}
				if err := tx.Save(&payment).Error; err != nil {
					return fmt.Errorf("failed to update payment amount: %v", err)
				}
			}
		}

		order.Subtotal = remainingSubtotal
		order.CouponDiscount = couponDiscount
		order.TotalPrice = order.Subtotal - couponAdjustemnt - order.OfferDiscount + order.ShippingCost
		if err := tx.Save(&order).Error; err != nil {
			return fmt.Errorf("failed to update order: %v", err)
		}

		if err := helper.UpdateProductStock(tx, cancelItem.ProductID); err != nil {
			return err
		}

		cancelItem.Status = "Cancelled"
		if err := tx.Save(cancelItem).Error; err != nil {
			return err
		}

		cancellation := userModels.Cancellation{OrderID: order.ID, ItemID: &cancelItem.ID, Reason: req.Reason}
		if err := tx.Save(&cancellation).Error; err != nil {
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
				if err := tx.Where("order_id = ? AND status IN ?", order.ID, []string{"Paid", "PartiallyRefundedToWallet"}).
					First(&payment).Error; err != nil {
					log.Printf("Payment details not found for order ID %d or not in refundable state: %v", order.ID, err)
				} else if payment.Amount > 0 {
					wallet, err := EnshureWallet(tx, uint(userID.(uint)))
					if err != nil {
						return err
					}
					wallet.Balance += payment.Amount
					if err := tx.Save(&wallet).Error; err != nil {
						return fmt.Errorf("failed to update wallet balance: %v", err)
					}
					payment.Status = "RefundedToWallet"
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

			order.Status = "Cancelled"
			if err := tx.Save(&order).Error; err != nil {
				return err
			}

		}

		return nil

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
    // 1. Extract user ID
    userID, exists := c.Get("id")
    if !exists {
        log.Printf("No user ID found in context")
        c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
        return
    }
    uid, ok := userID.(uint)
    if !ok {
        log.Printf("Invalid user ID type")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }

    // 2. Get order ID from URL parameter (this is the unique string ID)
    orderIDUnique := c.Param("order_id") // Renamed for clarity with unique ID
    if orderIDUnique == "" {
        log.Printf("No order ID provided")
        c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
        return
    }

    // 3. Fetch order with related data
    var order userModels.Orders
    if err := database.DB.Where("order_id_unique = ? AND user_id = ?", orderIDUnique, uid).
        Preload("OrderItems.Product").
        Preload("OrderItems.Variants").
        First(&order).Error; err != nil {
        log.Printf("Order not found for order_id_unique=%s, user_id=%d: %v", orderIDUnique, uid, err)
        c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
        return
    }

    // 4. Fetch shipping address
    var address adminModels.ShippingAddress
    // Use order.OrderIdUnique for consistency with how it's stored in ShippingAddress (assuming it's stored by the unique ID)
    if err := database.DB.Where("order_id = ? AND user_id = ?", order.OrderIdUnique, uid).First(&address).Error; err != nil {
        log.Printf("Shipping address not found for order_id=%s, user_id=%d: %v", order.OrderIdUnique, uid, err)
        c.JSON(http.StatusNotFound, gin.H{"error": "Shipping address not found"})
        return
    }

    // 5. Check if all items are cancelled and fetch backup data
    allOrderItemsCancelled := true
    for _, item := range order.OrderItems {
        if item.Status != "Cancelled" {
            allOrderItemsCancelled = false
            break
        }
    }

    var orderBackup userModels.OrderBackUp
    hasBackup := false

    // Fetch backup data only if all items are cancelled or the order itself is cancelled
    // or if the current order's total price is zero (implies effective cancellation)
    if allOrderItemsCancelled || order.Status == "Cancelled" || order.TotalPrice == 0 {
        if err := database.DB.Where("order_id_unique = ?", order.OrderIdUnique).First(&orderBackup).Error; err != nil {
            log.Printf("Order backup not found for order ID %d: %v", order.ID, err)
            hasBackup = false
        } else {
            hasBackup = true
        }
    }

    // 6. Calculate total offer discount from active order items for CURRENT display
    // If you want the original offer discount, use orderBackup.OfferDiscount when hasBackup is true.
    currentTotalOfferDiscount := 0.0
    for _, item := range order.OrderItems {
        if item.Status == "Active" { // Only sum discount for items that are still active
            currentTotalOfferDiscount += item.DiscountAmount
        }
    }


    // 7. Prepare user data for template
    user, exists := c.Get("user")
    userName, nameExists := c.Get("user_name")
    if !exists || !nameExists {
        c.HTML(http.StatusOK, "orderDetail.html", gin.H{
            "status":             "success",
            "Order":              order,
            "ShippingAddress":    address,
            "UserName":           "Guest",
            "WishlistCount":      0,
            "CartCount":          0,
            "ProfileImage":       "",
            "CurrentTotalOfferDiscount": currentTotalOfferDiscount, // Renamed for clarity
            "OrderBackup":        orderBackup,
            "HasBackup":          hasBackup,
            "AllItemsCancelled":  allOrderItemsCancelled, // Pass this flag to template
        })
        return
    }

    userData := user.(userModels.Users)
    userNameStr := userName.(string)

    // 8. Fetch wishlist and cart counts
    var wishlistCount, cartCount int64
    if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
        log.Printf("Failed to fetch wishlist count for user_id=%d: %v", userData.ID, err)
        wishlistCount = 0
    }
    if err := database.DB.Model(&userModels.CartItem{}).
        Joins("JOIN carts ON carts.id = cart_items.cart_id").
        Where("carts.user_id = ?", userData.ID).
        Count(&cartCount).Error; err != nil {
        log.Printf("Failed to fetch cart count for user_id=%d: %v", userData.ID, err)
        cartCount = 0
    }

    // 9. Render template
    c.HTML(http.StatusOK, "orderDetail.html", gin.H{
        "status":             "success",
        "Order":              order,
        "ShippingAddress":    address,
        "UserName":           userNameStr,
        "ProfileImage":       userData.ProfileImage,
        "WishlistCount":      wishlistCount,
        "CartCount":          cartCount,
        "CurrentTotalOfferDiscount": currentTotalOfferDiscount, // Renamed for clarity
        "OrderBackup":        orderBackup,
        "HasBackup":          hasBackup,
        "AllItemsCancelled":  allOrderItemsCancelled, // Pass this flag to template
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
		pdf.Cell(40, 10, fmt.Sprintf("%s (%s) - Qty: %d - $%.2f", item.Product.ProductName, item.Variants.Color, item.Quantity, item.UnitPrice*float64(item.Quantity)))
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

// func incrementStock(tx *gorm.DB, variantID, quantity uint) error {
// 	var variant adminModels.Variants
// 	if err := tx.First(&variant, variantID).Error; err != nil {
// 		return err
// 	}
// 	variant.Stock += quantity

// 	return tx.Save(&variant).Error
// }
