package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func GetProducts(c *gin.Context) {
	pkg.Log.Info("Handling request to get products")

	pageStr := c.Query("page")
	searchQuery := c.Query("search")
	pkg.Log.Debug("Received query parameters", zap.String("page", pageStr), zap.String("search", searchQuery))

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		pkg.Log.Warn("Invalid page number, defaulting to 1", zap.String("page", pageStr), zap.Error(err))
		page = 1
	}
	pkg.Log.Debug("Parsed page number", zap.Int("page", page))

	const itemsPerPage = 5
	offset := (page - 1) * itemsPerPage
	pkg.Log.Debug("Calculated pagination", zap.Int("items_per_page", itemsPerPage), zap.Int("offset", offset))

	var products []adminModels.Product
	var total int64
	dbQuery := database.DB.Model(&adminModels.Product{})

	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		dbQuery = dbQuery.Where("product_name ILIKE ? OR description ILIKE ? OR category_name ILIKE ?", searchPattern, searchPattern, searchPattern)
		pkg.Log.Debug("Applied search filter", zap.String("search_pattern", searchPattern))
	}

	if err := dbQuery.Count(&total).Error; err != nil {
		pkg.Log.Error("Failed to count products", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to count products", err.Error(), "")
		return
	}
	pkg.Log.Debug("Total products counted", zap.Int64("total", total))

	if err := dbQuery.Preload("Variants").Order("product_name").Offset(offset).Limit(itemsPerPage).Find(&products).Error; err != nil {
		pkg.Log.Error("Failed to fetch products", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch products", err.Error(), "")
		return
	}
	pkg.Log.Debug("Products retrieved", zap.Int("count", len(products)))

	type ProductWithStock struct {
		adminModels.Product
		TotalStock uint
	}

	var productsWithStock []ProductWithStock
	for _, product := range products {
		totalStock := uint(0)
		for _, variant := range product.Variants {
			totalStock += variant.Stock
		}
		productsWithStock = append(productsWithStock, ProductWithStock{
			Product:    product,
			TotalStock: totalStock,
		})
		pkg.Log.Debug("Calculated stock for product",
			zap.Uint("product_id", product.ID),
			zap.String("product_name", product.ProductName),
			zap.Uint("total_stock", totalStock))
	}

	totalPages := (int(total) + itemsPerPage - 1) / itemsPerPage
	pkg.Log.Debug("Calculated pagination", zap.Int("total_pages", totalPages))

	pkg.Log.Info("Rendering product.html",
		zap.Int("product_count", len(productsWithStock)),
		zap.Int("current_page", page),
		zap.Int("total_pages", totalPages),
		zap.String("search_query", searchQuery))
	c.HTML(http.StatusOK, "product.html", gin.H{
		"Products":    productsWithStock,
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"SearchQuery": searchQuery,
	})
}

func ToggleProductStatus(c *gin.Context) {
	pkg.Log.Info("Handling request to toggle product status")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		pkg.Log.Warn("Invalid product ID", zap.String("id", idStr), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid product ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed product ID", zap.Int("id", id))

	var product adminModels.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		pkg.Log.Error("Failed to find product", zap.Int("id", id), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Product not found", err.Error(), "")
		return
	}
	pkg.Log.Debug("Product retrieved", zap.Uint("id", product.ID), zap.String("product_name", product.ProductName))

	newStatus := !product.IsListed
	product.IsListed = newStatus
	if err := database.DB.Save(&product).Error; err != nil {
		pkg.Log.Error("Failed to toggle product status", zap.Uint("id", product.ID), zap.Bool("is_listed", newStatus), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to toggle product status", err.Error(), "")
		return
	}

	status := "listed"
	if !newStatus {
		status = "unlisted"
	}
	pkg.Log.Info("Product status toggled successfully",
		zap.Uint("id", product.ID),
		zap.String("product_name", product.ProductName),
		zap.Bool("is_listed", newStatus),
		zap.String("status", status))

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Product %s successfully", status),
	})
}