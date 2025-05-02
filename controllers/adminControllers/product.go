package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
)

func GetProducts(c *gin.Context) {
    pageStr := c.Query("page")
    searchQuery := c.Query("search")

    page, err := strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        page = 1
    }

    const itemsPerPage = 5
    offset := (page - 1) * itemsPerPage

    var products []adminModels.Product
    var total int64
    dbQuery := database.DB.Model(&adminModels.Product{})

    if searchQuery != "" {
        searchPattern := "%" + searchQuery + "%"
        dbQuery = dbQuery.Where("product_name ILIKE ? OR description ILIKE ? OR category_name ILIKE ?", searchPattern, searchPattern, searchPattern)
    }

    if err := dbQuery.Count(&total).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count products"})
        return
    }

    if err := dbQuery.Preload("Variants").Order("product_name").Offset(offset).Limit(itemsPerPage).Find(&products).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
        return
    }


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
    }

    totalPages := (int(total) + itemsPerPage - 1) / itemsPerPage

    c.HTML(http.StatusOK, "product.html", gin.H{
        "Products":    productsWithStock,
        "CurrentPage": page,
        "TotalPages":  totalPages,
        "SearchQuery": searchQuery,
    })
}

func ToggleProductStatus(c *gin.Context) {
    middleware.ClearCache()
    idStr := c.Param("id")
    id, err := strconv.Atoi(idStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
        return
    }

    var product adminModels.Product
    if err := database.DB.First(&product, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
        return
    }

    product.IsListed = !product.IsListed
    if err := database.DB.Save(&product).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle product status"})
        return
    }

    status := "listed"
    if !product.IsListed {
        status = "unlisted"
    }
    c.JSON(http.StatusOK, gin.H{
        "message": fmt.Sprintf("Product %s successfully", status),
    })
}