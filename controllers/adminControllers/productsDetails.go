package controllers

import (
	"net/http"
	"strconv"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type VariantOffer struct {
	Variant      adminModels.Variants
	OfferDetails helper.OfferDetails
}

func ProductDetailsHandler(c *gin.Context) {
	pkg.Log.Info("Handling request to get product details")

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		pkg.Log.Warn("Invalid product ID", zap.String("id", idStr), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid product ID", err.Error(), "")
		return
	}
	pkg.Log.Debug("Parsed product ID", zap.Uint64("id", id))

	var product adminModels.Product
	err = database.DB.Preload("Variants").First(&product, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			pkg.Log.Warn("Product not found", zap.Uint64("id", id))
			c.HTML(http.StatusOK, "productDetailsAdmin.html", gin.H{
				"Product": nil,
			})
			return
		}
		pkg.Log.Error("Failed to fetch product details", zap.Uint64("id", id), zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch product details", err.Error(), "")
		return
	}
	pkg.Log.Debug("Product retrieved",
		zap.Uint("id", product.ID),
		zap.String("product_name", product.ProductName),
		zap.Int("variant_count", len(product.Variants)))

	// Calculate offer details for the base product
	baseOfferDetails := helper.GetBestOfferForProduct(&product, 0.0)
	pkg.Log.Debug("Calculated base product offer",
		zap.Uint("product_id", product.ID),
		zap.Any("offer_details", baseOfferDetails))

	// Calculate offer details for each variant
	variantOffers := make([]VariantOffer, 0, len(product.Variants))
	for _, variant := range product.Variants {
		offerDetails := helper.GetBestOfferForProduct(&product, variant.ExtraPrice)
		variantOffers = append(variantOffers, VariantOffer{
			Variant:      variant,
			OfferDetails: offerDetails,
		})
		pkg.Log.Debug("Calculated variant offer",
			zap.Uint("variant_id", variant.ID),
			zap.String("color", variant.Color),
			zap.Any("offer_details", offerDetails))
	}

	pkg.Log.Info("Rendering productDetailsAdmin.html",
		zap.Uint("product_id", product.ID),
		zap.String("product_name", product.ProductName),
		zap.Int("variant_count", len(variantOffers)),
		zap.Any("base_offer_details", baseOfferDetails))
	c.HTML(http.StatusOK, "productDetailsAdmin.html", gin.H{
		"Product":       product,
		"OfferDetails":  baseOfferDetails,
		"VariantOffers": variantOffers,
	})
}