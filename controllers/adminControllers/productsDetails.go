package controllers

import (
    "net/http"
    "strconv"

    "github.com/Sojil8/eCommerce-silver/database"
    "github.com/Sojil8/eCommerce-silver/helper"
    "github.com/Sojil8/eCommerce-silver/models/adminModels"
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

type VariantOffer struct {
    Variant      adminModels.Variants
    OfferDetails helper.OfferDetails
}

func ProductDetailsHandler(c *gin.Context) {
    idStr := c.Param("id")
    id, err := strconv.ParseUint(idStr, 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
        return
    }

    var product adminModels.Product
    err = database.DB.Preload("Variants").First(&product, id).Error
    if err != nil {
        if err == gorm.ErrRecordNotFound {
            c.HTML(http.StatusOK, "productDetailsAdmin.html", gin.H{
                "Product": nil,
            })
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch product details"})
        return
    }

    // Calculate offer details for the base product
    baseOfferDetails := helper.GetBestOfferForProduct(&product, 0.0)

    // Calculate offer details for each variant
    variantOffers := make([]VariantOffer, 0, len(product.Variants))
    for _, variant := range product.Variants {
        offerDetails := helper.GetBestOfferForProduct(&product, variant.ExtraPrice)
        variantOffers = append(variantOffers, VariantOffer{
            Variant:      variant,
            OfferDetails: offerDetails,
        })
    }

    c.HTML(http.StatusOK, "productDetailsAdmin.html", gin.H{
        "Product":           product,
        "OfferDetails":      baseOfferDetails,
        "VariantOffers":     variantOffers,
        // "DiscountPercentage": discountPercentage, // Removed as unused
        // "RelatedProducts":   relatedProducts,     // Removed as unused
    })
}