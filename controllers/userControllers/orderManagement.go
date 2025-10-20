package controllers

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sojil8/eCommerce-silver/config"
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/services"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var req struct {
	Reason string `json:"reason"`
}

func GetOrderList(c *gin.Context) {
	userID, _ := c.Get("id")

	// Get query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit := 10
	offset := (page - 1) * limit
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort", "order_date desc")
	statusFilter := c.Query("status")

	// Build base query
	query := database.DB.Model(&userModels.Orders{}).Where("user_id = ?", userID)

	// Apply search filter
	if search != "" {
		// Enhanced search for order IDs - supports partial matches from beginning, middle, and end
		query = query.Where(`
			order_id_unique ILIKE ? OR 
			order_id_unique ILIKE ? OR 
			order_id_unique ILIKE ? OR 
			EXISTS (SELECT 1 FROM users WHERE users.id = orders.user_id AND (users.user_name ILIKE ? OR users.email ILIKE ?))`,
			search+"%",     // Starts with search term
			"%"+search,     // Ends with search term
			"%"+search+"%", // Contains search term
			"%"+search+"%", "%"+search+"%")
	}

	// Apply status filter
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	// Count total orders with filters applied
	var totalOrders int64
	query.Count(&totalOrders)

	// Build final query with preloads and sorting
	finalQuery := database.DB.Where("user_id = ?", userID).Preload("OrderItems.Product").Preload("User")

	// Apply same filters to final query
	if search != "" {
		finalQuery = finalQuery.Where(`
			order_id_unique ILIKE ? OR 
			order_id_unique ILIKE ? OR 
			order_id_unique ILIKE ? OR 
			EXISTS (SELECT 1 FROM users WHERE users.id = orders.user_id AND (users.user_name ILIKE ? OR users.email ILIKE ?))`,
			search+"%",     // Starts with search term
			"%"+search,     // Ends with search term
			"%"+search+"%", // Contains search term
			"%"+search+"%", "%"+search+"%")
	}

	if statusFilter != "" {
		finalQuery = finalQuery.Where("status = ?", statusFilter)
	}

	// Apply sorting
	switch sortBy {
	case "order_date desc":
		finalQuery = finalQuery.Order("order_date DESC")
	case "order_date asc":
		finalQuery = finalQuery.Order("order_date ASC")
	case "total_price desc":
		finalQuery = finalQuery.Order("total_price DESC")
	case "total_price asc":
		finalQuery = finalQuery.Order("total_price ASC")
	default:
		finalQuery = finalQuery.Order("order_date DESC")
	}

	// Apply pagination and execute query
	var orders []userModels.Orders
	if err := finalQuery.Limit(limit).Offset(offset).Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	totalPages := int(math.Ceil(float64(totalOrders) / float64(limit)))

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
			"TotalPages":    totalPages,
			"CurrentPage":   page,
			"Page":          page,
			"Total":         totalOrders,
			"Limit":         limit,
			"Search":        search,
			"Sort":          sortBy,
			"Filter":        statusFilter,
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
		"TotalPages":    totalPages,
		"CurrentPage":   page,
		"Page":          page,
		"Total":         totalOrders,
		"Limit":         limit,
		"Search":        search,
		"Sort":          sortBy,
		"Filter":        statusFilter,
	})
}

func ShowOrderDetails(c *gin.Context) {
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

	orderIDUnique := c.Param("order_id")
	if orderIDUnique == "" {
		log.Printf("No order ID provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	var order userModels.Orders
	if err := database.DB.Where("order_id_unique = ? AND user_id = ?", orderIDUnique, uid).
		Preload("OrderItems.Product").
		Preload("OrderItems.Variants").
		First(&order).Error; err != nil {
		log.Printf("Order not found for order_id_unique=%s, user_id=%d: %v", orderIDUnique, uid, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	var address adminModels.ShippingAddress
	if err := database.DB.Where("order_id = ? AND user_id = ?", order.OrderIdUnique, uid).First(&address).Error; err != nil {
		log.Printf("Shipping address not found for order_id=%s, user_id=%d: %v", order.OrderIdUnique, uid, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Shipping address not found"})
		return
	}

	allOrderItemsCancelled := true
	for _, item := range order.OrderItems {
		if item.Status != "Cancelled" {
			allOrderItemsCancelled = false
			break
		}
	}

	var orderBackup userModels.OrderBackUp
	hasBackup := false

	if allOrderItemsCancelled || order.Status == "Cancelled" || order.TotalPrice == 0 {
		if err := database.DB.Where("order_id_unique = ?", order.OrderIdUnique).First(&orderBackup).Error; err != nil {
			log.Printf("Order backup not found for order ID %d: %v", order.ID, err)
			hasBackup = false
		} else {
			hasBackup = true
		}
	}

	currentTotalOfferDiscount := 0.0
	for _, item := range order.OrderItems {
		if item.Status == "Active" {
			currentTotalOfferDiscount += item.DiscountAmount
		}
	}

	// Retrieve user data (following the exact pattern from GetUserProducts)
	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "orderDetail.html", gin.H{
			"status":                    "success",
			"Order":                     order,
			"ShippingAddress":           address,
			"UserName":                  "Guest",
			"WishlistCount":             0,
			"CartCount":                 0,
			"ProfileImage":              "",
			"CurrentTotalOfferDiscount": currentTotalOfferDiscount,
			"OrderBackup":               orderBackup,
			"HasBackup":                 hasBackup,
			"AllItemsCancelled":         allOrderItemsCancelled,
		})
		return
	}

	userData := user.(userModels.Users)
	userNameStr := userName.(string)

	// Fetch wishlist and cart counts (following the exact pattern from GetUserProducts)
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

	c.HTML(http.StatusOK, "orderDetail.html", gin.H{
		"status":                    "success",
		"Order":                     order,
		"ShippingAddress":           address,
		"UserName":                  userNameStr,           
		"ProfileImage":              userData.ProfileImage,
		"WishlistCount":             wishlistCount,         
		"CartCount":                 cartCount,            
		"CurrentTotalOfferDiscount": currentTotalOfferDiscount,
		"OrderBackup":               orderBackup,
		"HasBackup":                 hasBackup,
		"AllItemsCancelled":         allOrderItemsCancelled,
	})
}

func CancelOrder(c *gin.Context) {
	userID := helper.FetchUserID(c)
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

		if order.PaymentMethod == "ONLINE" || order.PaymentMethod == "WALLET" {
			var payment adminModels.PaymentDetails
			if err := tx.Where("order_id = ? AND status IN ?", order.ID, []string{"Paid", "Completed", "PartiallyRefundedToWallet"}).
				First(&payment).Error; err != nil {
				return fmt.Errorf("payment details not found: %v", err)
			}

			wallet, err := config.EnshureWallet(tx, uint(userID))
			if err != nil {
				return err
			}

			refundAmount := order.TotalPrice
			wallet.Balance += refundAmount
			if err := tx.Save(&wallet).Error; err != nil {
				return fmt.Errorf("failed to update wallet balance: %v", err)
			}
			walletTransaction := userModels.WalletTransaction{
				UserID:        userID,
				WalletID:      wallet.ID,
				Amount:        refundAmount,
				LastBalance:   wallet.Balance - refundAmount,
				Description:   fmt.Sprintf("Full refund for cancelled order %s", order.OrderIdUnique),
				Type:          "Credited",
				Receipt:       "rcpt-" + uuid.New().String(),
				OrderID:       order.OrderIdUnique,
				TransactionID: fmt.Sprintf("TXN-%d-%d", time.Now().UnixNano(), rand.Intn(10000)),
				PaymentMethod: order.PaymentMethod,
			}
			if err := tx.Create(&walletTransaction).Error; err != nil {
				pkg.Log.Error("Failed to create wallet transaction", zap.Uint("userID", userID), zap.Error(err))
				return fmt.Errorf("failed to create wallet transaction: %v", err)
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

		if order.CouponID != 0 {
			var coupons adminModels.Coupons
			if err := tx.First(&coupons, order.CouponID).Error; err != nil {
				return fmt.Errorf("failed to get the coupon details: %v", err)
			}
			coupons.UsedCount--
			if err := tx.Save(&coupons).Error; err != nil {
				return fmt.Errorf("failed to update coupon used count: %v", err)
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
	userIDAny, exists := c.Get("id")
	if !exists {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User not authenticated", "", "")
		return
	}
	userID, ok := userIDAny.(uint)
	if !ok {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID", "", "")
		return
	}

	orderID := c.Param("order_id")
	itemID := c.Param("item_id")

	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", "Please provide a valid reason", "")
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var order userModels.Orders
		if err := tx.Where("user_id = ? AND order_id_unique = ?", userID, orderID).
			Preload("OrderItems.Product").Preload("OrderItems.Variants").First(&order).Error; err != nil {
			pkg.Log.Error("Order not found", zap.Uint("userID", userID), zap.String("orderID", orderID), zap.Error(err))
			return fmt.Errorf("order not found: %v", err)
		}

		if order.Status != "Pending" && order.Status != "Confirmed" {
			return fmt.Errorf("only pending or confirmed orders can be modified")
		}

		if order.Status == "Cancelled" {
			return fmt.Errorf("order is already cancelled")
		}

		itemIDUint, err := strconv.ParseUint(itemID, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid item ID: %v", err)
		}

		var cancelItem *userModels.OrderItem
		for i, item := range order.OrderItems {
			if item.ID == uint(itemIDUint) {
				if item.Status != "Active" {
					return fmt.Errorf("item is already cancelled or not active")
				}
				cancelItem = &order.OrderItems[i]
				break
			}
		}
		if cancelItem == nil {
			return fmt.Errorf("item not found")
		}

		var variant adminModels.Variants
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&variant, cancelItem.VariantsID).Error; err != nil {
			pkg.Log.Error("Variant not found", zap.Uint("variantID", cancelItem.VariantsID), zap.Error(err))
			return fmt.Errorf("variant not found: %v", err)
		}

		variant.Stock += cancelItem.Quantity
		if err := tx.Save(&variant).Error; err != nil {
			pkg.Log.Error("Failed to update variant stock", zap.Uint("variantID", cancelItem.VariantsID), zap.Error(err))
			return fmt.Errorf("failed to update variant stock: %v", err)
		}

		// Simplified: Use totals directly to avoid floating-point division issues
		itemOriginalTotal := cancelItem.ItemTotal + cancelItem.DiscountAmount
		remainingSubtotal := order.Subtotal - itemOriginalTotal
		pkg.Log.Info("CancelItem",
			zap.Uint("itemID", cancelItem.ID),
			zap.Float64("subtotal", order.Subtotal),
			zap.Float64("cancelItemTotal", cancelItem.ItemTotal),
			zap.Float64("remainingSubtotal", remainingSubtotal))

		var coupon adminModels.Coupons
		couponDiscount := order.CouponDiscount
		var couponAdjustment float64

		// originalCouponID := order.CouponID // Store for potential decrement

		if order.CouponID != 0 {
			if err := tx.First(&coupon, order.CouponID).Error; err != nil {
				pkg.Log.Warn("Coupon not found", zap.Uint("couponID", order.CouponID), zap.Error(err))
				couponAdjustment = order.CouponDiscount
				couponDiscount = 0
				order.CouponID = 0
				order.CouponCode = ""
			} else if remainingSubtotal < coupon.MinPurchaseAmount {
				pkg.Log.Info("Coupon removed",
					zap.String("couponCode", coupon.CouponCode),
					zap.Float64("remainingSubtotal", remainingSubtotal),
					zap.Float64("minPurchaseAmount", coupon.MinPurchaseAmount))
				couponAdjustment = order.CouponDiscount
				couponDiscount = 0
				order.CouponID = 0
				order.CouponCode = ""
				// Fixed: Decrement coupon used count when removing
				coupon.UsedCount--
				if err := tx.Save(&coupon).Error; err != nil {
					return fmt.Errorf("failed to update coupon used count: %v", err)
				}
			} else {
				couponDiscount = remainingSubtotal * (coupon.DiscountPercentage / 100)
				couponAdjustment = order.CouponDiscount - couponDiscount
				pkg.Log.Info("Coupon adjusted",
					zap.String("couponCode", coupon.CouponCode),
					zap.Float64("oldDiscount", order.CouponDiscount),
					zap.Float64("newDiscount", couponDiscount))
			}
		}

		refundedAmount := cancelItem.ItemTotal - couponAdjustment
		var couponLoss float64
		if refundedAmount < 0 {
			couponLoss = -refundedAmount
			refundedAmount = 0
			pkg.Log.Info("Refund capped at 0, coupon loss calculated",
				zap.Float64("itemTotal", cancelItem.ItemTotal),
				zap.Float64("couponAdjustment", couponAdjustment),
				zap.Float64("couponLoss", couponLoss))
		}

		const maxNegativeBalance = -1000.0
		if order.PaymentMethod == "ONLINE" || order.PaymentMethod == "Wallet" {
			var payment adminModels.PaymentDetails
			if err := tx.Where("order_id = ?", order.ID).First(&payment).Error; err != nil && err != gorm.ErrRecordNotFound {
				pkg.Log.Error("Failed to fetch payment details", zap.Uint("orderID", order.ID), zap.Error(err))
				return fmt.Errorf("failed to fetch payment details: %v", err)
			}

			wallet, err := config.EnshureWallet(tx, userID)
			if err != nil {
				pkg.Log.Error("Failed to ensure wallet", zap.Uint("userID", userID), zap.Error(err))
				return fmt.Errorf("failed to ensure wallet: %v", err)
			}

			if couponLoss > 0 && wallet.Balance-couponLoss < maxNegativeBalance {
				pkg.Log.Warn("Negative balance exceeds limit",
					zap.Float64("currentBalance", wallet.Balance),
					zap.Float64("couponLoss", couponLoss),
					zap.Float64("maxNegativeBalance", maxNegativeBalance))
				return fmt.Errorf("deduction would exceed maximum negative balance of %.2f", maxNegativeBalance)
			}

			if refundedAmount > 0 {
				currentBalance := wallet.Balance
				wallet.Balance += refundedAmount
				if err := tx.Save(&wallet).Error; err != nil {
					pkg.Log.Error("Failed to update wallet balance for refund", zap.Uint("userID", userID), zap.Error(err))
					return fmt.Errorf("failed to update wallet balance: %v", err)
				}
				walletTransaction := userModels.WalletTransaction{
					UserID:        userID,
					WalletID:      wallet.ID,
					Amount:        refundedAmount,
					LastBalance:   currentBalance,
					Description:   fmt.Sprintf("Refund for cancelled item %s in order %s", cancelItem.Product.ProductName, order.OrderIdUnique),
					Type:          "Credited",
					Receipt:       "rcpt-" + uuid.New().String(),
					OrderID:       order.OrderIdUnique,
					TransactionID: fmt.Sprintf("TXN-%d-%d", time.Now().UnixNano(), rand.Intn(10000)),
					PaymentMethod: order.PaymentMethod,
				}
				if err := tx.Create(&walletTransaction).Error; err != nil {
					pkg.Log.Error("Failed to create wallet transaction for refund", zap.Uint("userID", userID), zap.Error(err))
					return fmt.Errorf("failed to create wallet transaction: %v", err)
				}
			}

			if couponLoss > 0 {
				currentBalance := wallet.Balance
				wallet.Balance -= couponLoss
				if err := tx.Save(&wallet).Error; err != nil {
					pkg.Log.Error("Failed to update wallet balance for coupon loss", zap.Uint("userID", userID), zap.Error(err))
					return fmt.Errorf("failed to update wallet balance for coupon loss: %v", err)
				}
				walletTransaction := userModels.WalletTransaction{
					UserID:        userID,
					WalletID:      wallet.ID,
					Amount:        couponLoss,
					LastBalance:   currentBalance,
					Description:   fmt.Sprintf("Coupon loss deduction for cancelled item %v in order %s", cancelItem.Product.ProductName, order.OrderIdUnique),
					Type:          "Debited",
					Receipt:       "rcpt-" + uuid.New().String(),
					OrderID:       order.OrderIdUnique,
					TransactionID: fmt.Sprintf("TXN-%d-%d", time.Now().UnixNano(), rand.Intn(10000)),
					PaymentMethod: order.PaymentMethod,
				}
				if err := tx.Create(&walletTransaction).Error; err != nil {
					pkg.Log.Error("Failed to create wallet transaction for coupon loss", zap.Uint("userID", userID), zap.Error(err))
					return fmt.Errorf("failed to create wallet transaction for coupon loss: %v", err)
				}
			}

			if payment.ID != 0 {
				payment.Amount -= refundedAmount
				if payment.Amount <= 0 {
					payment.Status = "RefundedToWallet"
				} else {
					payment.Status = "PartiallyRefundedToWallet"
				}
				if err := tx.Save(&payment).Error; err != nil {
					pkg.Log.Error("Failed to update payment amount", zap.Uint("orderID", order.ID), zap.Error(err))
					return fmt.Errorf("failed to update payment amount: %v", err)
				}
			}
		}

		order.Subtotal = remainingSubtotal
		order.CouponDiscount = couponDiscount
		order.OfferDiscount -= cancelItem.DiscountAmount
		order.TotalDiscount = order.OfferDiscount + order.CouponDiscount
		order.PaymentStatus = "Partialy Refunded"

		order.TotalPrice = order.Subtotal - order.CouponDiscount - order.OfferDiscount + order.ShippingCost

		if err := tx.Save(&order).Error; err != nil {
			pkg.Log.Error("Failed to update order", zap.Uint("orderID", order.ID), zap.Error(err))
			return fmt.Errorf("failed to update order: %v", err)
		}

		if err := helper.UpdateProductStock(tx, cancelItem.ProductID); err != nil {
			pkg.Log.Error("Failed to update product stock", zap.Uint("productID", cancelItem.ProductID), zap.Error(err))
			return fmt.Errorf("failed to update product stock: %v", err)
		}

		cancelItem.Status = "Cancelled"
		if err := tx.Save(cancelItem).Error; err != nil {
			pkg.Log.Error("Failed to update order item", zap.Uint("itemID", cancelItem.ID), zap.Error(err))
			return fmt.Errorf("failed to update order item: %v", err)
		}

		cancellation := userModels.Cancellation{OrderID: order.ID, ItemID: &cancelItem.ID, Reason: req.Reason}
		if err := tx.Create(&cancellation).Error; err != nil {
			pkg.Log.Error("Failed to create cancellation record", zap.Uint("orderID", order.ID), zap.Error(err))
			return fmt.Errorf("failed to create cancellation record: %v", err)
		}

		allCancelled := true
		for _, item := range order.OrderItems {
			if item.Status != "Cancelled" {
				allCancelled = false
				break
			}
		}

		if allCancelled {
			remainingRefund := order.TotalPrice
			if remainingRefund < 0 {
				pkg.Log.Error("Invalid remaining refund amount", zap.Float64("remainingRefund", remainingRefund), zap.String("orderID", order.OrderIdUnique))
				return fmt.Errorf("invalid remaining refund amount: %.2f", remainingRefund)
			}

			if remainingRefund > 0 && (order.PaymentMethod == "ONLINE" || order.PaymentMethod == "Wallet") {
				wallet, err := config.EnshureWallet(tx, userID)
				if err != nil {
					pkg.Log.Error("Failed to ensure wallet for full cancellation", zap.Uint("userID", userID), zap.Error(err))
					return fmt.Errorf("failed to ensure wallet: %v", err)
				}
				currentBalance := wallet.Balance
				wallet.Balance += remainingRefund
				if err := tx.Save(&wallet).Error; err != nil {
					pkg.Log.Error("Failed to update wallet balance for full cancellation", zap.Uint("userID", userID), zap.Error(err))
					return fmt.Errorf("failed to update wallet balance: %v", err)
				}
				walletTransaction := userModels.WalletTransaction{
					UserID:        userID,
					WalletID:      wallet.ID,
					Amount:        remainingRefund,
					LastBalance:   currentBalance,
					Description:   fmt.Sprintf("Full refund for cancelled order %s", order.OrderIdUnique),
					Type:          "Credited",
					Receipt:       "rcpt-" + uuid.New().String(),
					OrderID:       order.OrderIdUnique,
					TransactionID: fmt.Sprintf("TXN-%d-%d", time.Now().UnixNano(), rand.Intn(10000)),
					PaymentMethod: order.PaymentMethod,
				}
				if err := tx.Create(&walletTransaction).Error; err != nil {
					pkg.Log.Error("Failed to create wallet transaction for full refund", zap.Uint("userID", userID), zap.Error(err))
					return fmt.Errorf("failed to create wallet transaction: %v", err)
				}
			}

			var payment adminModels.PaymentDetails
			if err := tx.Where("order_id = ?", order.ID).First(&payment).Error; err == nil {
				payment.Status = "RefundedToWallet"
				payment.Amount = 0
				if err := tx.Save(&payment).Error; err != nil {
					pkg.Log.Error("Failed to update payment status for full cancellation", zap.Uint("orderID", order.ID), zap.Error(err))
					return fmt.Errorf("failed to update payment status: %v", err)
				}
			} else if err != gorm.ErrRecordNotFound {
				pkg.Log.Error("Failed to fetch payment details for full cancellation", zap.Uint("orderID", order.ID), zap.Error(err))
				return fmt.Errorf("failed to fetch payment details: %v", err)
			}

			// Fixed: Load backup and restore ALL fields for consistency (matches CancelOrder behavior)
			var orderBackUp userModels.OrderBackUp
			if err := tx.Where("order_id_unique = ?", orderID).First(&orderBackUp).Error; err != nil {
				return err
			}
			order.Subtotal = orderBackUp.Subtotal
			order.ShippingCost = orderBackUp.ShippingCost
			order.OfferDiscount = orderBackUp.OfferDiscount
			order.CouponDiscount = orderBackUp.CouponDiscount
			order.TotalDiscount = orderBackUp.OfferDiscount + orderBackUp.CouponDiscount
			order.TotalPrice = orderBackUp.TotalPrice
			order.Status = "Cancelled"
			if err := tx.Save(&order).Error; err != nil {
				pkg.Log.Error("Failed to update order status to Cancelled", zap.Uint("orderID", order.ID), zap.Error(err))
				return fmt.Errorf("failed to update order status: %v", err)
			}

			orderCancellation := userModels.Cancellation{OrderID: order.ID, Reason: req.Reason}
			if err := tx.Create(&orderCancellation).Error; err != nil {
				pkg.Log.Error("Failed to create order cancellation record", zap.Uint("orderID", order.ID), zap.Error(err))
				return fmt.Errorf("failed to create order cancellation record: %v", err)
			}
		}

		return nil
	})

	if err != nil {
		pkg.Log.Error("Failed to cancel item", zap.String("orderID", orderID), zap.String("itemID", itemID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Failed to cancel item", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Item cancelled successfully",
	})
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

func RetryPayment(c *gin.Context) {
	userID := helper.FetchUserID(c)
	orderID := c.Param("order_id")

	var order userModels.Orders
	if err := database.DB.Preload("OrderItems").Where("order_id_unique = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
		pkg.Log.Error("Order not found", zap.String("orderID", orderID), zap.Uint("userID", userID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Order not found", "Order not found", "/orders")
		return
	}

	if order.PaymentStatus != "Failed" && order.PaymentStatus != "Pending" {
		pkg.Log.Warn("Payment retry not allowed", zap.String("orderID", orderID), zap.String("paymentStatus", order.PaymentStatus))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Payment retry not allowed", "Order payment status does not allow retry", "/orders")
		return
	}

	if order.PaymentMethod != "ONLINE" {
		pkg.Log.Warn("Invalid payment method for retry", zap.String("method", order.PaymentMethod))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid payment method", "Only Razorpay payments can be retried", "/orders")
		return
	}

	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		pkg.Log.Error("User not found", zap.Uint("userID", userID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "User not found", "/orders")
		return
	}

	var shippingAddress adminModels.ShippingAddress
	if err := database.DB.Where("order_id = ? AND user_id = ?", orderID, userID).First(&shippingAddress).Error; err != nil {
		pkg.Log.Error("Shipping address not found", zap.String("orderID", orderID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid shipping address", "Shipping address not found", "/orders")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Unexpected error", "Something went wrong", "/order/failure")
		}
	}()

	stockOk := true
	for _, item := range order.OrderItems {
		var variant adminModels.Variants
		if err := tx.First(&variant, item.VariantsID).Error; err != nil {
			pkg.Log.Error("Variant not found", zap.Uint("variantsID", item.VariantsID), zap.Error(err))
			stockOk = false
			break
		}
		if variant.Stock < item.Quantity {
			pkg.Log.Error("Insufficient stock", zap.Uint("variantsID", item.VariantsID), zap.Uint("available", variant.Stock), zap.Uint("required", item.Quantity))
			stockOk = false
			break
		}
	}
	if !stockOk {
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusBadRequest, "Insufficient stock", "One or more products are out of stock", "/orders")
		return
	}

	amountInPaise := int(math.Round(order.TotalPrice * 100))

	razorpayOrder, err := services.CreateRazorpayOrder(amountInPaise)
	if err != nil {
		pkg.Log.Error("Failed to create Razorpay order", zap.Float64("amount", order.TotalPrice), zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create Razorpay order", "Something went wrong", "/order/failure")
		return
	}

	razorpayOrderID, ok := razorpayOrder["id"].(string)
	if !ok {
		pkg.Log.Error("Failed to extract Razorpay order ID", zap.Any("razorpayOrder", razorpayOrder))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid Razorpay response", "Something went wrong", "/order/failure")
		return
	}

	order.RazorpayOrderID = razorpayOrderID
	order.PaymentStatus = "Pending"
	order.OrderError = ""
	if err := tx.Save(&order).Error; err != nil {
		pkg.Log.Error("Failed to update order with Razorpay ID", zap.String("razorpayOrderID", razorpayOrderID), zap.Error(err))
		tx.Rollback()
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process Razorpay", "Something went wrong", "/order/failure")
		return
	}

	var payment adminModels.PaymentDetails
	if err := tx.Where("order_id = ? AND payment_method = ?", order.ID, "ONLINE").First(&payment).Error; err == nil {
		payment.RazorpayOrderID = razorpayOrderID
		payment.Status = "Pending"
		payment.Attempts++
		payment.CreatedAt = time.Now()
		if err := tx.Save(&payment).Error; err != nil {
			pkg.Log.Error("Failed to update payment record", zap.Uint("orderID", order.ID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update payment record", "Something went wrong", "/order/failure")
			return
		}
	} else {
		payment = adminModels.PaymentDetails{
			OrderID:         order.ID,
			UserID:          userID,
			AddressID:       order.AddressID,
			PaymentMethod:   "ONLINE",
			Amount:          order.TotalPrice,
			Status:          "Pending",
			RazorpayOrderID: razorpayOrderID,
			Attempts:        1,
			CreatedAt:       time.Now(),
		}
		if err := tx.Create(&payment).Error; err != nil {
			pkg.Log.Error("Failed to create payment record", zap.Uint("orderID", order.ID), zap.Error(err))
			tx.Rollback()
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create payment record", "Something went wrong", "/order/failure")
			return
		}
	}

	tx.Commit()
	pkg.Log.Info("Razorpay retry initiated", zap.String("razorpayOrderID", razorpayOrderID), zap.Uint("userID", userID))
	c.JSON(http.StatusOK, gin.H{
		"status":            "payment_required",
		"key_id":            os.Getenv("RAZORPAY_KEY_ID"),
		"razorpay_order_id": razorpayOrderID,
		"amount":            int(order.TotalPrice * 100),
		"currency":          "INR",
		"order_id":          order.OrderIdUnique,
		"prefill": gin.H{
			"name":    user.UserName,
			"email":   user.Email,
			"contact": shippingAddress.Phone,
		},
		"notes": gin.H{
			"address":  shippingAddress.City,
			"user_id":  userID,
			"order_id": order.ID,
		},
	})
}
