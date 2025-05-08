package controllers

import (

	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)


func ProductDetailsHandler(c *gin.Context) {
    idStr := c.Param("id")
    id, errr := strconv.ParseUint(idStr, 10, 32)
    if errr != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
        return
    }

    var product adminModels.Product
    err := database.DB.Preload("Variants").First(&product, id).Error
    if err != nil {
        if err == gorm.ErrRecordNotFound {
            c.HTML(http.StatusOK, "productDetails.html", gin.H{
                "Product": nil,
            })
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch product details"})
        return
    }

 

    c.HTML(http.StatusOK, "productDetailsAdmin.html", gin.H{
        "Product":           product,
        // "DiscountPercentage": discountPercentage,
        // "RelatedProducts":   relatedProducts,
    })
}