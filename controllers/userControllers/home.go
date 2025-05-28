package controllers

import (
	"log"
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
)

func GetUserProducts(c *gin.Context) {

	db := database.DB.Model(&adminModels.Product{}).
		Joins("JOIN categories ON categories.category_name = products.category_name").
		Where("products.is_listed=? AND categories.status = ?", true, true).
		Preload("Variants")

	var products []adminModels.Product
	if err := db.Find(&products).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "error:failed to fetch products", "error:failed to fetch products", "")
		return
	}

	if products == nil {
		log.Println("product are not retreving")
	}

	type ProductWithOffer struct {
		adminModels.Product
		OfferPrice    float64
		OriginalPrice float64
		DiscountPercentage float64
		IsOffer       bool
		OfferName     string
	}

	var produtWithOffers []ProductWithOffer

	for _, product := range products {
		variantExtraPrice := 0.0
		if len(product.Variants) > 0 {
			variantExtraPrice = product.Variants[0].ExtraPrice
		}

		offer :=helper.GetBestOfferForProduct(&product,variantExtraPrice)

		produtWithOffers =append(produtWithOffers,ProductWithOffer{
			Product: product,
			OfferPrice:offer.DiscountedPrice ,
			OriginalPrice: offer.OriginalPrice,
			DiscountPercentage: offer.DiscountPercentage,
			IsOffer: offer.IsOfferApplied,
			OfferName: offer.OfferName,
		})

	}	


	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		c.HTML(http.StatusOK, "home.html", gin.H{
			"status":        "success",
			"Products":      produtWithOffers,
			"UserName":      "Guest",
			"WishlistCount": 0,
			"CartCount":     0,
			"ProfileImage":  "",
		})
		return
	}

	userData := user.(userModels.Users)
	userNameStr := userName.(string)

	var wishlistCount, cartCount int64
	if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		cartCount = 0
	}

	c.HTML(http.StatusOK, "home.html", gin.H{
		"status":        "success",
		"Products":      produtWithOffers,
		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
	})
}
