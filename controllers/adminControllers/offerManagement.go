package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type offerRequest struct {
	OfferName string    `json:"offer_name" binding:"required"`
	Discount  float64   `json:"discount" binding:"required,gt=0"`
	StartDate time.Time `json:"start_date" binding:"required"`
	EndDate   time.Time `json:"end_date" binding:"required"`
	IsActive  bool      `json:"is_active"`
}

func ShowOfferPage(c *gin.Context) {
	pkg.Log.Info("Handling request to show offer page")

	var categories []adminModels.Category
	if err := database.DB.Find(&categories).Error; err != nil {
		pkg.Log.Error("Failed to load categories", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to load categories", err.Error(), "")
		return
	}
	pkg.Log.Debug("Categories loaded", zap.Int("count", len(categories)))

	var products []adminModels.Product
	if err := database.DB.Find(&products).Error; err != nil {
		pkg.Log.Error("Failed to load products", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to load products", err.Error(), "")
		return
	}
	pkg.Log.Debug("Products loaded", zap.Int("count", len(products)))

	var categoryOffers []adminModels.CategoryOffer
	if err := database.DB.Preload("Category").Find(&categoryOffers).Error; err != nil {
		pkg.Log.Error("Failed to load category offers", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to load category offers", err.Error(), "")
		return
	}
	pkg.Log.Debug("Category offers loaded", zap.Int("count", len(categoryOffers)))

	var productOffers []adminModels.ProductOffer
	if err := database.DB.Preload("Product").Find(&productOffers).Error; err != nil {
		pkg.Log.Error("Failed to load product offers", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to load product offers", err.Error(), "")
		return
	}
	pkg.Log.Debug("Product offers loaded", zap.Int("count", len(productOffers)))

	pkg.Log.Info("Rendering offerManagement.html",
		zap.Int("category_count", len(categories)),
		zap.Int("product_count", len(products)),
		zap.Int("category_offer_count", len(categoryOffers)),
		zap.Int("product_offer_count", len(productOffers)))
	c.HTML(http.StatusOK, "offerManagement.html", gin.H{
		"ProductOffers":  productOffers,
		"CategoryOffers": categoryOffers,
		"products":       products,
		"categories":     categories,
	})
}

func AddProductOffer(c *gin.Context) {
	pkg.Log.Info("Handling request to add product offer")

	productIDStr := c.Param("product_id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		pkg.Log.Warn("Invalid product ID", zap.String("product_id", productIDStr), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid product ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed product ID", zap.Int("product_id", productID))

	var req offerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Log.Error("Failed to bind JSON data", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Failed to bind data", err.Error(), "")
		return
	}
	pkg.Log.Debug("Received product offer data",
		zap.String("offer_name", req.OfferName),
		zap.Float64("discount", req.Discount),
		zap.Time("start_date", req.StartDate),
		zap.Time("end_date", req.EndDate),
		zap.Bool("is_active", req.IsActive))

	var productOffer adminModels.ProductOffer
	if err := database.DB.Where("product_id = ?", productID).First(&productOffer).Error; err == nil {
		pkg.Log.Warn("Product offer already exists", zap.Int("product_id", productID))
		helper.ResponseWithErr(c, http.StatusConflict, "Offer already exists", "Only one offer allowed per product", "")
		return
	}

	if req.Discount >= 60 {
		pkg.Log.Warn("Discount too high", zap.Float64("discount", req.Discount))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Discount too high", "Discount must be less than 60%", "")
		return
	}

	if req.EndDate.Before(req.StartDate) {
		pkg.Log.Warn("Invalid date range", zap.Time("start_date", req.StartDate), zap.Time("end_date", req.EndDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid date range", "End date must be after start date", "")
		return
	}

	today := time.Now().Truncate(24 * time.Hour)
	if req.StartDate.Before(today) {
		pkg.Log.Warn("Invalid start date", zap.Time("start_date", req.StartDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid start date", "Start date must be today or in the future", "")
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
		pkg.Log.Error("Failed to create product offer", zap.Int("product_id", productID), zap.String("offer_name", req.OfferName), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to save offer", err.Error(), "")
		return
	}

	pkg.Log.Info("Product offer created successfully", zap.Uint("offer_id", offer.ID), zap.String("offer_name", offer.OfferName))
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func AddCategoryOffer(c *gin.Context) {
	pkg.Log.Info("Handling request to add category offer")

	categoryIDStr := c.Param("category_id")
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		pkg.Log.Warn("Invalid category ID", zap.String("category_id", categoryIDStr), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid category ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed category ID", zap.Int("category_id", categoryID))

	var req offerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Log.Error("Failed to bind JSON data", zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Failed to bind data", err.Error(), "")
		return
	}
	pkg.Log.Debug("Received category offer data",
		zap.String("offer_name", req.OfferName),
		zap.Float64("discount", req.Discount),
		zap.Time("start_date", req.StartDate),
		zap.Time("end_date", req.EndDate),
		zap.Bool("is_active", req.IsActive))

	if req.Discount >= 60 {
		pkg.Log.Warn("Discount too high", zap.Float64("discount", req.Discount))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Discount too high", "Discount must be less than 60%", "")
		return
	}

	if req.EndDate.Before(req.StartDate) {
		pkg.Log.Warn("Invalid date range", zap.Time("start_date", req.StartDate), zap.Time("end_date", req.EndDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid date range", "End date must be after start date", "")
		return
	}

	today := time.Now().Truncate(24 * time.Hour)
	if req.StartDate.Before(today) {
		pkg.Log.Warn("Invalid start date", zap.Time("start_date", req.StartDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid start date", "Start date must be today or in the future", "")
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
		pkg.Log.Error("Failed to create category offer", zap.Int("category_id", categoryID), zap.String("offer_name", req.OfferName), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to save offer", err.Error(), "")
		return
	}

	pkg.Log.Info("Category offer created successfully", zap.Uint("offer_id", offer.ID), zap.String("offer_name", offer.OfferName))
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func ShowEditProductOffer(c *gin.Context) {
	pkg.Log.Info("Handling request to show edit product offer")

	productIDStr := c.Param("id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		pkg.Log.Warn("Invalid product offer ID", zap.String("id", productIDStr), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid product offer ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed product offer ID", zap.Int("id", productID))

	var productOffer adminModels.ProductOffer
	if err := database.DB.First(&productOffer, productID).Error; err != nil {
		pkg.Log.Error("Failed to find product offer", zap.Int("id", productID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Product offer not found", err.Error(), "")
		return
	}
	pkg.Log.Debug("Product offer retrieved", zap.Uint("id", productOffer.ID), zap.String("offer_name", productOffer.OfferName))

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"offer": gin.H{
			"offer_name": productOffer.OfferName,
			"discount":   productOffer.Discount,
			"start_date": productOffer.StartDate,
			"end_date":   productOffer.EndDate,
			"is_active":  productOffer.IsActive,
		},
	})
}

func EditProductOffer(c *gin.Context) {
	pkg.Log.Info("Handling request to edit product offer")

	productIDStr := c.Param("id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		pkg.Log.Warn("Invalid product offer ID", zap.String("id", productIDStr), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid product offer ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed product offer ID", zap.Int("id", productID))

	var req offerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Log.Error("Failed to bind JSON data", zap.Int("id", productID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Failed to bind data", err.Error(), "")
		return
	}
	pkg.Log.Debug("Received product offer update data",
		zap.String("offer_name", req.OfferName),
		zap.Float64("discount", req.Discount),
		zap.Time("start_date", req.StartDate),
		zap.Time("end_date", req.EndDate),
		zap.Bool("is_active", req.IsActive))

	var productOffer adminModels.ProductOffer
	if err := database.DB.First(&productOffer, productID).Error; err != nil {
		pkg.Log.Error("Failed to find product offer", zap.Int("id", productID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Product offer not found", err.Error(), "")
		return
	}
	pkg.Log.Debug("Product offer retrieved", zap.Uint("id", productOffer.ID), zap.String("offer_name", productOffer.OfferName))

	if req.OfferName == "" || req.Discount == 0 || req.StartDate.IsZero() || req.EndDate.IsZero() {
		pkg.Log.Warn("Missing required fields", zap.Any("request", req))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Missing required fields", "All fields are required", "")
		return
	}

	if req.Discount >= 60 {
		pkg.Log.Warn("Discount too high", zap.Float64("discount", req.Discount))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Discount too high", "Discount must be less than 60%", "")
		return
	}

	if req.EndDate.Before(req.StartDate) {
		pkg.Log.Warn("Invalid date range", zap.Time("start_date", req.StartDate), zap.Time("end_date", req.EndDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid date range", "End date must be after start date", "")
		return
	}

	if req.EndDate == req.StartDate {
		pkg.Log.Warn("Offer duration too short", zap.Time("start_date", req.StartDate), zap.Time("end_date", req.EndDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid date range", "Offer must be at least one day", "")
		return
	}

	today := time.Now().Truncate(24 * time.Hour)
	if req.StartDate.Before(today) {
		pkg.Log.Warn("Invalid start date", zap.Time("start_date", req.StartDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid start date", "Start date must be today or in the future", "")
		return
	}

	productOffer.OfferName = req.OfferName
	productOffer.Discount = req.Discount
	productOffer.StartDate = req.StartDate
	productOffer.EndDate = req.EndDate
	productOffer.IsActive = req.IsActive

	if err := database.DB.Save(&productOffer).Error; err != nil {
		pkg.Log.Error("Failed to update product offer", zap.Uint("id", productOffer.ID), zap.String("offer_name", req.OfferName), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to save product offer", err.Error(), "")
		return
	}

	pkg.Log.Info("Product offer updated successfully", zap.Uint("id", productOffer.ID), zap.String("offer_name", productOffer.OfferName))
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"offer": gin.H{
			"offer_name": productOffer.OfferName,
			"discount":   productOffer.Discount,
			"start_date": productOffer.StartDate,
			"end_date":   productOffer.EndDate,
			"is_active":  productOffer.IsActive,
		},
	})
}

func DeleteProductOffer(c *gin.Context) {
	pkg.Log.Info("Handling request to delete product offer")

	productIDStr := c.Param("id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		pkg.Log.Warn("Invalid product offer ID", zap.String("id", productIDStr), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid product offer ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed product offer ID", zap.Int("id", productID))

	var productOffer adminModels.ProductOffer
	if err := database.DB.First(&productOffer, productID).Error; err != nil {
		pkg.Log.Error("Product offer not found", zap.Int("id", productID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Product offer not found", err.Error(), "")
		return
	}
	pkg.Log.Debug("Product offer retrieved", zap.Uint("id", productOffer.ID), zap.String("offer_name", productOffer.OfferName))

	if err := database.DB.Delete(&productOffer).Error; err != nil {
		pkg.Log.Error("Failed to delete product offer", zap.Uint("id", productOffer.ID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to delete product offer", err.Error(), "")
		return
	}

	pkg.Log.Info("Product offer deleted successfully", zap.Uint("id", productOffer.ID), zap.String("offer_name", productOffer.OfferName))
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func ShowCategoryOfferEdit(c *gin.Context) {
	pkg.Log.Info("Handling request to show edit category offer")

	categoryIDStr := c.Param("id")
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		pkg.Log.Warn("Invalid category offer ID", zap.String("id", categoryIDStr), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid category offer ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed category offer ID", zap.Int("id", categoryID))

	var categoryOffer adminModels.CategoryOffer
	if err := database.DB.First(&categoryOffer, categoryID).Error; err != nil {
		pkg.Log.Error("Failed to find category offer", zap.Int("id", categoryID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Category offer not found", err.Error(), "")
		return
	}
	pkg.Log.Debug("Category offer retrieved", zap.Uint("id", categoryOffer.ID), zap.String("offer_name", categoryOffer.OfferName))

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"offer": gin.H{
			"offer_name": categoryOffer.OfferName,
			"discount":   categoryOffer.Discount,
			"start_date": categoryOffer.StartDate,
			"end_date":   categoryOffer.EndDate,
			"is_active":  categoryOffer.IsActive,
		},
	})
}

func EditCategoryOffer(c *gin.Context) {
	pkg.Log.Info("Handling request to edit category offer")

	categoryIDStr := c.Param("id")
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		pkg.Log.Warn("Invalid category offer ID", zap.String("id", categoryIDStr), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid category offer ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed category offer ID", zap.Int("id", categoryID))

	var req offerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Log.Error("Failed to bind JSON data", zap.Int("id", categoryID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Failed to bind data", err.Error(), "")
		return
	}
	pkg.Log.Debug("Received category offer update data",
		zap.String("offer_name", req.OfferName),
		zap.Float64("discount", req.Discount),
		zap.Time("start_date", req.StartDate),
		zap.Time("end_date", req.EndDate),
		zap.Bool("is_active", req.IsActive))

	var categoryOffer adminModels.CategoryOffer
	if err := database.DB.First(&categoryOffer, categoryID).Error; err != nil {
		pkg.Log.Error("Failed to find category offer", zap.Int("id", categoryID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Category offer not found", err.Error(), "")
		return
	}
	pkg.Log.Debug("Category offer retrieved", zap.Uint("id", categoryOffer.ID), zap.String("offer_name", categoryOffer.OfferName))

	if req.OfferName == "" || req.Discount == 0 || req.StartDate.IsZero() || req.EndDate.IsZero() {
		pkg.Log.Warn("Missing required fields", zap.Any("request", req))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Missing required fields", "All fields are required", "")
		return
	}

	if req.Discount >= 60 {
		pkg.Log.Warn("Discount too high", zap.Float64("discount", req.Discount))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Discount too high", "Discount must be less than 60%", "")
		return
	}

	if req.EndDate.Before(req.StartDate) {
		pkg.Log.Warn("Invalid date range", zap.Time("start_date", req.StartDate), zap.Time("end_date", req.EndDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid date range", "End date must be after start date", "")
		return
	}

	if req.EndDate == req.StartDate {
		pkg.Log.Warn("Offer duration too short", zap.Time("start_date", req.StartDate), zap.Time("end_date", req.EndDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid date range", "Offer must be at least one day", "")
		return
	}

	today := time.Now().Truncate(24 * time.Hour)
	if req.StartDate.Before(today) {
		pkg.Log.Warn("Invalid start date", zap.Time("start_date", req.StartDate))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid start date", "Start date must be today or in the future", "")
		return
	}

	categoryOffer.OfferName = req.OfferName
	categoryOffer.Discount = req.Discount
	categoryOffer.StartDate = req.StartDate
	categoryOffer.EndDate = req.EndDate
	categoryOffer.IsActive = req.IsActive

	if err := database.DB.Save(&categoryOffer).Error; err != nil {
		pkg.Log.Error("Failed to update category offer", zap.Uint("id", categoryOffer.ID), zap.String("offer_name", req.OfferName), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to save category offer", err.Error(), "")
		return
	}

	pkg.Log.Info("Category offer updated successfully", zap.Uint("id", categoryOffer.ID), zap.String("offer_name", categoryOffer.OfferName))
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"offer": gin.H{
			"offer_name": categoryOffer.OfferName,
			"discount":   categoryOffer.Discount,
			"start_date": categoryOffer.StartDate,
			"end_date":   categoryOffer.EndDate,
			"is_active":  categoryOffer.IsActive,
		},
	})
}

func DeleteCategoryOffer(c *gin.Context) {
	pkg.Log.Info("Handling request to delete category offer")

	categoryIDStr := c.Param("id")
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		pkg.Log.Warn("Invalid category offer ID", zap.String("id", categoryIDStr), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid category offer ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed category offer ID", zap.Int("id", categoryID))

	var categoryOffer adminModels.CategoryOffer
	if err := database.DB.First(&categoryOffer, categoryID).Error; err != nil {
		pkg.Log.Error("Category offer not found", zap.Int("id", categoryID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Category offer not found", err.Error(), "")
		return
	}
	pkg.Log.Debug("Category offer retrieved", zap.Uint("id", categoryOffer.ID), zap.String("offer_name", categoryOffer.OfferName))

	if err := database.DB.Delete(&categoryOffer).Error; err != nil {
		pkg.Log.Error("Failed to delete category offer", zap.Uint("id", categoryOffer.ID), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to delete category offer", err.Error(), "")
		return
	}

	pkg.Log.Info("Category offer deleted successfully", zap.Uint("id", categoryOffer.ID), zap.String("offer_name", categoryOffer.OfferName))
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
