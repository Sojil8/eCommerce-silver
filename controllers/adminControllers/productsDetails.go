package controllers

// import (
// 	"fmt"
// 	"net/http"
// 	"strconv"

// 	"github.com/Sojil8/eCommerce-silver/database"
// 	"github.com/Sojil8/eCommerce-silver/models/adminModels"
// 	"github.com/gin-gonic/gin"
// 	"gorm.io/gorm"
// )

// // func ProductListHandler(c *gin.Context) {
// // 	pageStr := c.DefaultQuery("page", "1")
// // 	searchQuery := c.Query("search")

// // 	page, err := strconv.Atoi(pageStr)
// // 	if err != nil || page < 1 {
// // 		page = 1
// // 	}

// // 	limit := 10 // Products per page
// // 	offset := (page - 1) * limit

// // 	var products []adminModels.Product
// // 	query := database.DB.Limit(limit).Offset(offset)

// // 	if searchQuery != "" {
// // 		query = query.Where("product_name LIKE ? OR description LIKE ? OR category_name LIKE ?",
// // 			"%"+searchQuery+"%", "%"+searchQuery+"%", "%"+searchQuery+"%")
// // 	}

// // 	err = query.Find(&products).Error
// // 	if err != nil {
// // 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
// // 		return
// // 	}

// // 	c.HTML(http.StatusOK, "product_listing.html", gin.H{
// // 		"Products":    products,
// // 		"SearchQuery": searchQuery,
// // 	})
// // }

// func ProductDetailsHandler(c *gin.Context) {
//     idStr := c.Param("id")
//     id, err := strconv.ParseUint(idStr, 10, 32)
//     if err != nil {
//         c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
//         return
//     }

//     var product adminModels.Product
//     err = database.DB.Preload("Variants").First(&product, id).Error
//     if err != nil {
//         if err == gorm.ErrRecordNotFound {
//             c.HTML(http.StatusOK, "productDetails.html", gin.H{
//                 "Product": nil,
//             })
//             return
//         }
//         c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch product details"})
//         return
//     }

//     // Calculate discount percentage (if OriginalPrice exists)
//     var discountPercentage int
//     if product.OriginalPrice > product.Price && product.OriginalPrice > 0 {
//         discountPercentage = int(((product.OriginalPrice - product.Price) / product.OriginalPrice) * 100)
//     }

//     // Fetch related products (optional for admin side)
//     var relatedProducts []adminModels.Product
//     err = database.DB.Where("category_name = ? AND id != ? AND is_listed = ?", 
//         product.CategoryName, product.ID, true).Limit(4).Find(&relatedProducts).Error
//     if err != nil {
//         fmt.Println("Error fetching related products:", err)
//     }

//     // Debug: Print product data
//     fmt.Printf("Product: %+v\n", product)
//     fmt.Printf("Variants: %+v\n", product.Variants)
//     fmt.Printf("Discount: %d%%\n", discountPercentage)

//     c.HTML(http.StatusOK, "productDetails.html", gin.H{
//         "Product":           product,
//         "DiscountPercentage": discountPercentage,
//         "RelatedProducts":   relatedProducts,
//     })
// }