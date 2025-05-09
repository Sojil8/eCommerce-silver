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

type offerRequest struct {
	OfferName string
	Discount  float64
	StartDate time.Time
	EndDate   time.Time
	IsActive  bool
}

func ShowOfferPage(c *gin.Context) {
	
	var categorys []adminModels.Category
	if err:=database.DB.Find(&categorys).Error;err!=nil{
		helper.ResponseWithErr(c,http.StatusNotFound,"faild to load categorys","faild to load categorys","")
		return
	}
	if categorys == nil{

	}

	var products []adminModels.Product
	if err:=database.DB.Find(&products).Error;err!=nil{
		helper.ResponseWithErr(c,http.StatusNotFound,"faild to load products","faild to load products","")
		return
	}



	var categoryOffers []adminModels.CategoryOffer
	if err := database.DB.Preload("Category").Find(&categoryOffers).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to list category offers", err.Error(), "")
		return
	}

	var productOfffers []adminModels.ProductOffer
	if err := database.DB.Preload("Product").Find(&productOfffers).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to list product offers", err.Error(), "")
		return
	}

	c.HTML(http.StatusOK, "offerManagement.html", gin.H{
		"ProductOffers":  productOfffers,
		"CategoryOffers": categoryOffers,
		"products":products,
		"categories":categorys,
	})
}

func AddProductOffer(c *gin.Context){
	var productIDStr = c.Param("product_id")
	productID,_:=strconv.Atoi(productIDStr)

	var req offerRequest
	if err:= c.ShouldBindJSON(&req);err!=nil{
		helper.ResponseWithErr(c,http.StatusBadRequest,"faild to bind data","Faild to Bind Product offer","")
		return
	}

	// if req.OfferName != " "{
	// 	helper.ResponseWithErr(c,http.StatusBadRequest,"Offer Name should Be charaters","Offer Name should Be charaters","")
	// 	return
	// }

	if req.Discount >=60 {
		helper.ResponseWithErr(c,http.StatusBadRequest,"Discount should be less than 60","Discount should be less than 60","")
		return
	}
	if req.EndDate.Before(req.StartDate){
		helper.ResponseWithErr(c,http.StatusBadRequest,"End date Should be Greater than Start date","End date Should be Greater than Start date","")
		return
	}

	if req.StartDate.Before(time.Now()){
		helper.ResponseWithErr(c,http.StatusBadRequest,"start date Should be in future","start date Should be in future","")
		return
	}

	offer := adminModels.ProductOffer{
		ProductID: uint(productID),
		OfferName: req.OfferName,
		Discount: req.Discount,
		StartDate: req.StartDate,
		EndDate: req.EndDate,
		IsActive: req.IsActive,
	}
	if err:=database.DB.Create(&offer).Error;err!=nil{
		helper.ResponseWithErr(c,http.StatusInternalServerError,"Error while saving Data","Error while saving Data","")
		return
	}
	
	c.JSON(http.StatusOK,gin.H{
		"status":"ok",
	})
}


func AddCategoryOffer(c *gin.Context){
	var categoryIDStr = c.Param("category_id")
	categoryID,_:=strconv.Atoi(categoryIDStr)

	var req offerRequest
	if err:= c.ShouldBindJSON(&req);err!=nil{
		helper.ResponseWithErr(c,http.StatusBadRequest,"faild to bind data","Faild to Bind Product offer","")
		return
	}

	if req.Discount >=60 {
		helper.ResponseWithErr(c,http.StatusBadRequest,"Discount should be less than 60","Discount should be less than 60","")
		return
	}
	if req.EndDate.Before(req.StartDate){
		helper.ResponseWithErr(c,http.StatusBadRequest,"End date Should be Greater than Start date","End date Should be Greater than Start date","")
		return
	}

	if req.StartDate.Before(time.Now()){
		helper.ResponseWithErr(c,http.StatusBadRequest,"start date Should be in future","start date Should be in future","")
		return
	}

	offer := adminModels.CategoryOffer{
		CategoryID: uint(categoryID),
		OfferName: req.OfferName,
		Discount: req.Discount,
		StartDate: req.StartDate,
		EndDate: req.EndDate,
		IsActive: req.IsActive,
	}
	if err:=database.DB.Create(&offer).Error;err!=nil{
		helper.ResponseWithErr(c,http.StatusInternalServerError,"Error while saving Data","Error while saving Data","")
		return
	}
	
	c.JSON(http.StatusOK,gin.H{
		"status":"ok",
	})
}





func ShowEditProductOffer(c *gin.Context) {
	productIDStr := c.Param("id")
	productID, _ := strconv.Atoi(productIDStr)

	var produtOffer adminModels.ProductOffer
	if err := database.DB.First(&produtOffer, productID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to Product ", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"Status": "ok",
		"offer": gin.H{
			"offer_name":      produtOffer.OfferName,
			"discount":        produtOffer.Discount,
			"start_date":      produtOffer.StartDate,
			"end_date":        produtOffer.EndDate,
			"offer.is_active": produtOffer.IsActive,
		},
	})

}

func EditProductOffer(c *gin.Context) {
	productIDStr := c.Param("id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid offer ID", err.Error(), "")
		return
	}

	var req offerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Failed to bind ProductOffer in edit productOffer", err.Error(), "")
		return

	}

	var productOffer adminModels.ProductOffer
	if err := database.DB.First(&productOffer, productID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to list product offers", err.Error(), "")
		return
	}

	productOffer.OfferName = req.OfferName
	productOffer.Discount = req.Discount
	productOffer.StartDate = req.StartDate
	productOffer.EndDate = req.EndDate
	productOffer.IsActive = req.IsActive

	if req.OfferName == "" || req.Discount == 0 || req.StartDate.IsZero() || req.EndDate.IsZero() {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Missing required fields", "All fields are required", "")
		return
	}

	if req.Discount >= 60 {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Discount should be less than 60", "Discount should be less than 60", "")
		return
	}

	if req.EndDate.Before(req.StartDate) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "End date want to be greater than start date", "End date want to be greater than start date", "")
		return
	}

	if req.EndDate == req.StartDate {
		helper.ResponseWithErr(c, http.StatusBadRequest, "offer should be atlest one day", "offer should be atlest one day", "")
		return
	}

	if req.StartDate.Before(time.Now()) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "start date want to be in future", "start date want to be in future", "")
		return
	}

	if err := database.DB.Save(&productOffer).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to save product offers", err.Error(), "")
		return
	}

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

func DeleteProductOffer(c *gin.Context){
	productIDStr:=c.Param("id")
	productID,_:=strconv.Atoi(productIDStr)

	var productOffer adminModels.ProductOffer
	if err:=database.DB.Delete(&productOffer,productID).Error;err!=nil{
		helper.ResponseWithErr(c,http.StatusNotFound,"Offer Not found","Offer Not found","")
		return
	}

	c.JSON(http.StatusOK,gin.H{
		"status":"ok",
	})

}

func ShowCategoryOfferEdit(c *gin.Context) {
	categoryIDStr := c.Param("id")
	categoryID, _ := strconv.Atoi(categoryIDStr)

	var categoryOffer adminModels.CategoryOffer
	if err := database.DB.Find(&categoryOffer, categoryID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to get the Category offer", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"offer": gin.H{
			"offer_name": categoryOffer.OfferName,
			"discount":   categoryOffer.Discount,
			"start_date": categoryOffer.StartDate,
			"end_date":   categoryOffer.EndDate,
		},
	})
}

func EditCategoryOffer(c *gin.Context){
	categoryIDStr := c.Param("id")
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid offer ID", err.Error(), "")
		return
	}

	var req offerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Failed to bind ProductOffer in edit productOffer", err.Error(), "")
		return

	}

	var CategoryOffer adminModels.ProductOffer
	if err := database.DB.First(&CategoryOffer, categoryID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Failed to list product offers", err.Error(), "")
		return
	}

	CategoryOffer.OfferName = req.OfferName
	CategoryOffer.Discount = req.Discount
	CategoryOffer.StartDate = req.StartDate
	CategoryOffer.EndDate = req.EndDate
	CategoryOffer.IsActive = req.IsActive

	if req.OfferName == "" || req.Discount == 0 || req.StartDate.IsZero() || req.EndDate.IsZero() {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Missing required fields", "All fields are required", "")
		return
	}

	if req.Discount >= 60 {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Discount should be less than 60", "Discount should be less than 60", "")
		return
	}

	if req.EndDate.Before(req.StartDate) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "End date want to be greater than start date", "End date want to be greater than start date", "")
		return
	}

	if req.EndDate == req.StartDate {
		helper.ResponseWithErr(c, http.StatusBadRequest, "offer should be atlest one day", "offer should be atlest one day", "")
		return
	}

	if req.StartDate.Before(time.Now()) {
		helper.ResponseWithErr(c, http.StatusBadRequest, "start date want to be in future", "start date want to be in future", "")
		return
	}

	if err := database.DB.Save(&CategoryOffer).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to save product offers", err.Error(), "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"offer": gin.H{
			"offer_name": CategoryOffer.OfferName,
			"discount":   CategoryOffer.Discount,
			"start_date": CategoryOffer.StartDate,
			"end_date":   CategoryOffer.EndDate,
			"is_active":  CategoryOffer.IsActive,
		},
	})
}

func DeleteCategoryOffer(c *gin.Context){
	categoryIDStr:=c.Param("id")
	categoryID,_:=strconv.Atoi(categoryIDStr)

	var categoryOffer adminModels.ProductOffer
	if err:=database.DB.Delete(&categoryOffer,categoryID).Error;err!=nil{
		helper.ResponseWithErr(c,http.StatusNotFound,"Offer Not found","Offer Not found","")
		return
	}

	c.JSON(http.StatusOK,gin.H{
		"status":"ok",
	})

}