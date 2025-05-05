package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/gin-gonic/gin"
)

type OfferRequest struct {
	OfferName string    `json:"offer_name" binding:"required,max=100"`
	Discount  float64   `json:"discount" binding:"required,gt=0,lte=100"`
	StartDate time.Time `json:"start_date" binding:"required"`
	EndDate   time.Time `json:"end_date" binding:"required"`
	IsActive  bool      `json:"is_active"`
}

// GetAllCategories returns a list of all categories in JSON format
// func GetAllCategories(c *gin.Context) {
// 	var categories []adminModels.Category
// 	if err := database.DB.Find(&categories).Error; err != nil {
// 		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch categories", err.Error(), "")
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"status":     "ok",
// 		"categories": categories,
// 	})
// }

// GetAllProducts returns a list of all products in JSON format
// func GetAllProducts(c *gin.Context) {
// 	var products []adminModels.Product
// 	if err := database.DB.Find(&products).Error; err != nil {
// 		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch products", err.Error(), "")
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"status":   "ok",
// 		"products": products,
// 	})
// }

// ShowOfferPage displays the unified page with products and offers
func ShowOfferPage(c *gin.Context) {
	var products []adminModels.Product
	var productOffers []struct {
		adminModels.ProductOffer
		ProductName string `json:"product_name"`
	}
	var categoryOffers []struct {
		adminModels.CategoryOffer
		CategoryName string `json:"category_name"`
	}
	var categories []adminModels.Category

	// Fetch all products (non-deleted)
	if err := database.DB.Where("deleted_at IS NULL").Find(&products).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to list products", err.Error(), "")
		return
	}

	// Fetch all product offers with product names (non-deleted)
	if err := database.DB.
		Table("product_offers").
		Select("product_offers.*, products.product_name").
		Joins("JOIN products ON products.id = product_offers.product_id").
		Where("product_offers.deleted_at IS NULL").
		Find(&productOffers).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to list product offers", err.Error(), "")
		return
	}

	// Fetch all category offers with category names (non-deleted)
	if err := database.DB.
		Table("category_offers").
		Select("category_offers.*, categories.category_name").
		Joins("JOIN categories ON categories.id = category_offers.category_id").
		Where("category_offers.deleted_at IS NULL").
		Find(&categoryOffers).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to list category offers", err.Error(), "")
		return
	}

	// Fetch all categories (non-deleted)
	if err := database.DB.Where("deleted_at IS NULL").Find(&categories).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to list categories", err.Error(), "")
		return
	}

	// Render the unified page
	c.HTML(http.StatusOK, "offerManagement.html", gin.H{
		"status":         "ok",
		"Products":       products,
		"ProductOffers":  productOffers,
		"CategoryOffers": categoryOffers,
		"Categories":     categories,
	})
}

func AddProductOffer(c *gin.Context) {
	var req OfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	productIDStr := c.Param("product_id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid product ID", err.Error(), "")
		return
	}

	if req.EndDate.Before(req.StartDate) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid dates", "End date must be after start date", "")
		return
	}

	if req.StartDate.Before(time.Now()) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid start date", "Start date must be in the future", "")
		return
	}

	var product adminModels.Product
	if err := database.DB.First(&product, productID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Product not found", err.Error(), "")
		return
	}

	offer := adminModels.ProductOffer{
		ProductID: uint(productID),
		OfferName: req.OfferName,
		Discount:  req.Discount,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		IsActive:  req.IsActive,
	}

	if err := database.DB.Create(&offer).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot insert data to the database", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Product offer added successfully",
		"offer": gin.H{
			"product_id": offer.ProductID,
			"offer_name": offer.OfferName,
			"discount":   offer.Discount,
			"start_date": offer.StartDate,
			"end_date":   offer.EndDate,
			"is_active":  offer.IsActive,
		},
	})
}

func GetProductOffer(c *gin.Context) {
	id := c.Param("id")
	offerID, err := strconv.Atoi(id)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid offer ID", err.Error(), "")
		return
	}

	var offer adminModels.ProductOffer
	if err := database.DB.First(&offer, offerID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Product offer not found", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"offer": gin.H{
			"product_id":  offer.ProductID,
			"offer_name":  offer.OfferName,
			"discount":    offer.Discount,
			"start_date":  offer.StartDate,
			"end_date":    offer.EndDate,
			"is_active":   offer.IsActive,
		},
	})
}

func EditProductOffer(c *gin.Context) {
	offerIDStr := c.Param("id")
	offerID, err := strconv.Atoi(offerIDStr)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid offer ID", err.Error(), "")
		return
	}

	var req OfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	var offer adminModels.ProductOffer
	if err := database.DB.First(&offer, offerID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Product offer not found", err.Error(), "")
		return
	}

	if req.EndDate.Before(req.StartDate) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid dates", "End date must be after start date", "")
		return
	}

	offer.OfferName = req.OfferName
	offer.Discount = req.Discount
	offer.StartDate = req.StartDate
	offer.EndDate = req.EndDate
	offer.IsActive = req.IsActive

	if err := database.DB.Save(&offer).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot update the offer", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Product offer updated successfully",
		"offer": gin.H{
			"product_id": offer.ProductID,
			"offer_name": offer.OfferName,
			"discount":   offer.Discount,
			"start_date": offer.StartDate,
			"end_date":   offer.EndDate,
			"is_active":  offer.IsActive,
		},
	})
}

func DeleteProductOffer(c *gin.Context) {
	id := c.Param("id")
	offerID, err := strconv.Atoi(id)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid offer ID", err.Error(), "")
		return
	}

	var offer adminModels.ProductOffer
	if err := database.DB.First(&offer, offerID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Product offer not found", err.Error(), "")
		return
	}

	if err := database.DB.Delete(&offer).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot delete the offer", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Product offer deleted successfully",
	})
}

func AddCategoryOffer(c *gin.Context) {
	var req OfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	categoryIDStr := c.Param("category_id")
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid category ID", err.Error(), "")
		return
	}

	if req.EndDate.Before(req.StartDate) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid dates", "End date must be after start date", "")
		return
	}

	if req.StartDate.Before(time.Now()) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid start date", "Start date must be in the future", "")
		return
	}

	var category adminModels.Category
	if err := database.DB.First(&category, categoryID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Category not found", err.Error(), "")
		return
	}

	offer := adminModels.CategoryOffer{
		CategoryID: uint(categoryID),
		OfferName:  req.OfferName,
		Discount:   req.Discount,
		StartDate:  req.StartDate,
		EndDate:    req.EndDate,
		IsActive:   req.IsActive,
	}

	if err := database.DB.Create(&offer).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot insert data to the database", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Category offer added successfully",
		"offer": gin.H{
			"category_id": offer.CategoryID,
			"offer_name":  offer.OfferName,
			"discount":    offer.Discount,
			"start_date":  offer.StartDate,
			"end_date":    offer.EndDate,
			"is_active":   offer.IsActive,
		},
	})
}

func GetCategoryOffer(c *gin.Context) {
	id := c.Param("id")
	offerID, err := strconv.Atoi(id)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid offer ID", err.Error(), "")
		return
	}

	var offer adminModels.CategoryOffer
	if err := database.DB.First(&offer, offerID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Category offer not found", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"offer": gin.H{
			"category_id": offer.CategoryID,
			"offer_name":  offer.OfferName,
			"discount":    offer.Discount,
			"start_date":  offer.StartDate,
			"end_date":    offer.EndDate,
			"is_active":   offer.IsActive,
		},
	})
}

func EditCategoryOffer(c *gin.Context) {
	offerIDStr := c.Param("id")
	offerID, err := strconv.Atoi(offerIDStr)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid offer ID", err.Error(), "")
		return
	}

	var req OfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid request", err.Error(), "")
		return
	}

	var offer adminModels.CategoryOffer
	if err := database.DB.First(&offer, offerID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Category offer not found", err.Error(), "")
		return
	}

	if req.EndDate.Before(req.StartDate) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid dates", "End date must be after start date", "")
		return
	}

	offer.OfferName = req.OfferName
	offer.Discount = req.Discount
	offer.StartDate = req.StartDate
	offer.EndDate = req.EndDate
	offer.IsActive = req.IsActive

	if err := database.DB.Save(&offer).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot update the offer", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Category offer updated successfully",
		"offer": gin.H{
			"category_id": offer.CategoryID,
			"offer_name":  offer.OfferName,
			"discount":    offer.Discount,
			"start_date":  offer.StartDate,
			"end_date":    offer.EndDate,
			"is_active":   offer.IsActive,
		},
	})
}

func DeleteCategoryOffer(c *gin.Context) {
	id := c.Param("id")
	offerID, err := strconv.Atoi(id)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid offer ID", err.Error(), "")
		return
	}

	var offer adminModels.CategoryOffer
	if err := database.DB.First(&offer, offerID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Category offer not found", err.Error(), "")
		return
	}

	if err := database.DB.Delete(&offer).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Cannot delete the offer", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Category offer deleted successfully",
	})
}

// ApplyBestOffer applies the largest valid offer (product or category) to a product
// func ApplyBestOffer(c *gin.Context) {
// 	productIDStr := c.Param("product_id")
// 	productID, err := strconv.Atoi(productIDStr)
// 	if err != nil {
// 		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid product ID", err.Error(), "")
// 		return
// 	}

// 	var product adminModels.Product
// 	if err := database.DB.First(&product, productID).Error; err != nil {
// 		helper.ResponseWithErr(c, http.StatusNotFound, "Product not found", err.Error(), "")
// 		return
// 	}

// 	var productOffer adminModels.ProductOffer
// 	var categoryOffer adminModels.CategoryOffer
// 	var category adminModels.Category

// 	// Fetch product offer
// 	productOfferDiscount := 0.0
// 	if err := database.DB.Where("product_id = ? AND is_active = ? AND start_date <= ? AND end_date >= ?",
// 		productID, true, time.Now(), time.Now()).First(&productOffer).Error; err == nil {
// 		productOfferDiscount = productOffer.Discount
// 	}

// 	// Fetch category offer
// 	categoryOfferDiscount := 0.0
// 	if err := database.DB.Where("category_name = ?", product.CategoryName).First(&category).Error; err == nil {
// 		if err := database.DB.Where("category_id = ? AND is_active = ? AND start_date <= ? AND end_date >= ?",
// 			category.ID, true, time.Now(), time.Now()).First(&categoryOffer).Error; err == nil {
// 			categoryOfferDiscount = categoryOffer.Discount
// 		}
// 	}

// 	// Determine the best offer
// 	bestDiscount := 0.0
// 	offerType := "none"
// 	if productOfferDiscount > categoryOfferDiscount {
// 		bestDiscount = productOfferDiscount
// 		offerType = "product"
// 	} else if categoryOfferDiscount > 0 {
// 		bestDiscount = categoryOfferDiscount
// 		offerType = "category"
// 	}

// 	discountedPrice := product.Price
// 	if bestDiscount > 0 {
// 		discountedPrice = product.Price * (1 - bestDiscount/100)
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"status": "ok",
// 		"product": gin.H{
// 			"product_id":       product.ID,
// 			"product_name":     product.ProductName,
// 			"original_price":   product.Price,
// 			"discounted_price": discountedPrice,
// 			"applied_discount": bestDiscount,
// 			"offer_type":       offerType,
// 		},
// 	})
// }
