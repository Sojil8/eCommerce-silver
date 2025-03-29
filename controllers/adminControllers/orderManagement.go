package controllers

import (
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
)

func ListOrder(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	sort := c.Query("sort", "order_date desc")
	filterStatus := c.Query("status")

	offset := (page - 1) * limit
	var orders []userModels.Orders
	query := database.DB.Preload("OrderItems.Product").Preload("OrderItems.Variants").Preload("User")
}
