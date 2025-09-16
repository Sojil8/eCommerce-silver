package services

import (
	"fmt"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func FetchCartByUserID(userID uint) (*userModels.Cart, error) {
	var cart userModels.Cart
	if err := database.DB.Preload("CartItems").Preload("CartItems.Product").Preload("CartItems.Variants").First(&cart, "user_id = ?", userID).Error; err != nil {
		pkg.Log.Error("Failed to fetch cart", zap.Uint("userID", userID), zap.Error(err))
		return nil, err
	}
	return &cart, nil
}

func ValidateCartItems(cart *userModels.Cart, tx *gorm.DB) (totalPrice, originalTotalPrice, offerDiscount float64, validCartItems []userModels.CartItem, err error) {
	totalPrice = 0.0
	originalTotalPrice = 0.0
	offerDiscount = 0.0
	validCartItems = []userModels.CartItem{}

	for _, item := range cart.CartItems {
		var category adminModels.Category
		var variant adminModels.Variants

		isInStock := item.Product.IsListed && item.Product.InStock
		hasVariantStock := false
		if err := tx.Where("id = ? AND deleted_at IS NULL", item.VariantsID).First(&variant).Error; err == nil {
			hasVariantStock = variant.Stock >= item.Quantity
		}

		if isInStock && hasVariantStock && tx.Where("category_name = ? AND status = ?", item.Product.CategoryName, true).First(&category).Error == nil {
			offerDetails := helper.GetBestOfferForProduct(&item.Product, item.Variants.ExtraPrice)
			item.Price = offerDetails.OriginalPrice
			item.DiscountedPrice = offerDetails.DiscountedPrice
			item.RegularPrice = offerDetails.OriginalPrice
			item.OfferDiscountPercentage = offerDetails.DiscountPercentage
			item.OfferName = offerDetails.OfferName
			item.IsOfferApplied = offerDetails.IsOfferApplied
			item.SalePrice = offerDetails.DiscountedPrice * float64(item.Quantity)
			totalPrice += item.SalePrice
			originalTotalPrice += offerDetails.OriginalPrice * float64(item.Quantity)
			itemOfferDiscount := (offerDetails.OriginalPrice - offerDetails.DiscountedPrice) * float64(item.Quantity)
			offerDiscount += itemOfferDiscount
			validCartItems = append(validCartItems, item)
		} else {
			pkg.Log.Warn("Skipping invalid cart item",
				zap.Uint("productID", item.ProductID),
				zap.Uint("variantID", item.VariantsID),
				zap.Bool("productListed", item.Product.IsListed),
				zap.Bool("productInStock", item.Product.InStock),
				zap.Uint("variantStock", variant.Stock),
				zap.Uint("quantity", item.Quantity))
		}
	}

	if len(validCartItems) == 0 {
		return 0, 0, 0, nil, fmt.Errorf("no valid items in cart with sufficient stock")
	}

	return totalPrice, originalTotalPrice, offerDiscount, validCartItems, nil
}



func ClearCart(userID uint, tx *gorm.DB) error {
	var cart userModels.Cart
	if err := tx.First(&cart, "user_id = ?", userID).Error; err != nil {
		pkg.Log.Error("Cart not found for clearing", zap.Uint("userID", userID), zap.Error(err))
		return err
	}
	if err := tx.Where("cart_id = ?", cart.ID).Delete(&userModels.CartItem{}).Error; err != nil {
		pkg.Log.Error("Failed to delete cart items", zap.Uint("cartID", cart.ID), zap.Error(err))
		return err
	}
	if err := tx.Delete(&cart).Error; err != nil {
		pkg.Log.Error("Failed to delete cart", zap.Uint("cartID", cart.ID), zap.Error(err))
		return err
	}
	return nil
}