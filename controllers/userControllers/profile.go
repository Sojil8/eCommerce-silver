package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func ShowProfile(c *gin.Context) {
	userID, _ := c.Get("id")
	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Profile not available", "")
		return
	}

	var orders []userModels.Orders
	if err := database.DB.Where("user_id=?", userID).Find(&orders).Error; err != nil {
		log.Println("Error fetching orders:", err)
	}
	
	var addresses []userModels.Address
	if err:=database.DB.Where("user_id = ?",userID).Find(&addresses).Error;err!=nil{
		log.Println("Error fetching orders:", err)
	}

	c.HTML(http.StatusOK, "profile.html", gin.H{
		"title":       "User Profile",
		"User":        user,
		"Orders":      orders,
		"Addresses":   addresses,
		"profile_img": user.ProfileImage,
	})
}

func ShowEditProfile(c *gin.Context) {
	userID, _ := c.Get("id")
	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Invalid user ID", "")
		return
	}
	c.HTML(http.StatusOK, "editProfile.html", gin.H{
		"User": user,
	})
}

var editProfileRequest struct {
	UserName  string `form:"username" binding:"required"`
	FirstName string `form:"first_name"`
	LastName  string `form:"last_name"`
	Email     string `form:"email"`
	Phone     string `form:"phone"`
}

func EditProfile(c *gin.Context) {
	userID, _ := c.Get("id")
	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Profile not available", "")
		return
	}

	if err := c.ShouldBind(&editProfileRequest); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please check all fields", "")
		return
	}
	var imgURL string
	file, header, err := c.Request.FormFile("profile_img")
	if err == nil {
		defer file.Close()
		imgURL, err = helper.ProcessImage(c, file, header)
		if err != nil {
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to upload image", err.Error(), "")
			return
		}
	}

	if editProfileRequest.Email != user.Email {
		otp, err := helper.GenerateAndStoreOTP(editProfileRequest.Email)
		if err != nil {
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to generate OTP", "Internal error", "")
			return
		}

		tempData := map[string]interface{}{
			"username":    editProfileRequest.UserName,
			"first_name":  editProfileRequest.FirstName,
			"last_name":   editProfileRequest.LastName,
			"email":       editProfileRequest.Email,
			"phone":       editProfileRequest.Phone,
			"otp":         otp,
			"profile_img": imgURL,
		}

		data, _ := json.Marshal(tempData)
		key := fmt.Sprintf("edit:%d", userID)
		database.RedisClient.Set(database.Ctx, key, data, 15*time.Minute)

		if err := helper.SendOTP(editProfileRequest.Email, otp); err != nil {
			database.RedisClient.Del(database.Ctx, key)
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to send OTP", "Email sending failed", "")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "OTP sent to new email",
		})
		return
	}

	if imgURL != "" {
		user.ProfileImage = imgURL
	}
	user.UserName = editProfileRequest.UserName
	user.First_name = editProfileRequest.FirstName
	user.Last_name = editProfileRequest.LastName
	user.Phone = editProfileRequest.Phone

	if err := database.DB.Save(&user).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update profile", "Database error", "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Profile updated successfully",
	})
}

func ShowVerifyEditEmail(c *gin.Context) {
	email := c.Query("email")
	c.HTML(http.StatusOK, "verifyEmail.html", gin.H{
		"Email": email,
	})
}

var verifyEditEmailRequest struct {
	Email string `form:"email" binding:"required,email"`
	OTP   string `form:"otp" binding:"required,len=6"`
}

func VerifyEditEmail(c *gin.Context) {
	userID, _ := c.Get("id")
	if err := c.ShouldBind(&verifyEditEmailRequest); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please provide a valid OTP", "")
		return
	}
	key := fmt.Sprintf("edit:%d", userID)
	data, err := database.RedisClient.Get(database.Ctx, key).Result()
	if err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "OTP expired or not found", "Session expired", "")
		return
	}

	var tempData map[string]interface{}
	json.Unmarshal([]byte(data), &tempData)
	if tempData["otp"] != verifyEditEmailRequest.OTP {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid OTP", "OTP does not match", "")
		return
	}

	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Profile not available", "")
		return
	}

	user.UserName = tempData["username"].(string)
	user.First_name = tempData["first_name"].(string)
	user.Last_name = tempData["last_name"].(string)
	user.Email = tempData["email"].(string)
	user.Phone = tempData["phone"].(string)

	if imgURL, ok := tempData["profile_img"].(string); ok && imgURL != "" {
		user.ProfileImage = imgURL
	}

	if err := database.DB.Save(&user).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update profile", "Database error", "")
		return
	}
	database.RedisClient.Del(database.Ctx, key)
	c.Redirect(http.StatusSeeOther, "/profile")
}

var changePasswordRequest struct {
	CurrentPassword string `form:"current_password" binding:"required"`
	NewPassword     string `form:"new_password" binding:"required"`
	ConfirmPassword string `form:"confirm_password" binding:"required"`
}

func ChangePassword(c *gin.Context) {
	userID, _ := c.Get("id")
	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Profile not available", "")
		return
	}
	if err := c.ShouldBind(&changePasswordRequest); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please check all fields", "")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(changePasswordRequest.CurrentPassword)); err != nil {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "Incorrect current password", "Password mismatch", "")
		return
	}

	if changePasswordRequest.NewPassword != changePasswordRequest.ConfirmPassword {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Passwords do not match", "Please ensure new passwords match", "")
		return
	}

	hashedPassword, err := helper.HashPassword(changePasswordRequest.NewPassword)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to hash password", "Internal error", "")
		return
	}

	user.Password = hashedPassword
	if err := database.DB.Save(&user).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update password", "Database error", "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok", "message": "Password changed successfully",
	})
}

var addressRequest struct {
	AddressType    string `json:"address_type" binding:"required"`
	Name           string `json:"name" binding:"required"`
	City           string `json:"city" binding:"required"`
	Landmark       string `json:"landmark"`
	State          string `json:"state" binding:"required"`
	Pincode        string `json:"pincode" binding:"required"`
	Phone          string `json:"phone" binding:"required"`
	AlternatePhone string `json:"alternate_phone"`
}

func AddAddress(c *gin.Context) {
	userID, _ := c.Get("id")
	if err := c.ShouldBindJSON(&addressRequest); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please check all fields", "")
		return
	}

	address:=userModels.Address{
		UserID: userID.(uint),
		AddressType: addressRequest.AddressType,
		Name: addressRequest.Name,
		City: addressRequest.City,
		Landmark: addressRequest.Landmark,
		State: addressRequest.State,
		Pincode: addressRequest.Pincode,
		Phone: addressRequest.Phone,
		AlternatePhone: addressRequest.AlternatePhone,
	}
	if err:=database.DB.Create(&address).Error;err!=nil{
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to add address", "Database error", "")
        return
	}

	c.JSON(http.StatusOK,gin.H{
		"status":  "ok",
        "message": "Address added successfully",
	})
}


func EditAddress(c *gin.Context){
	userID,_:=c.Get("id")
	addressID:=c.Param("address_id")
	if err:=c.ShouldBind(&addressRequest);err!=nil{
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please check all fields", "")
        return
	}

	var address userModels.Address
	if err:=database.DB.Where("id = ? AND user_id = ?",addressID,userID).First(&address).Error;err!=nil{
		helper.ResponseWithErr(c, http.StatusNotFound, "Address not found", "Invalid address ID", "")
        return
	}

	address.AddressType=addressRequest.AddressType
	address.Name = addressRequest.Name
    address.City = addressRequest.City
    address.Landmark = addressRequest.Landmark
    address.State = addressRequest.State
    address.Pincode = addressRequest.Pincode
    address.Phone = addressRequest.Phone
    address.AlternatePhone = addressRequest.AlternatePhone	

	if err:=database.DB.Save(&address).Error;err!=nil{
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update address", "Database error", "")
        return
	}

	c.JSON(http.StatusOK, gin.H{
        "status":  "ok",
        "message": "Address updated successfully",
    })
}

func DeleteAddress(c *gin.Context){
	userID,_:=c.Get("id")
	addressID:=c.Param("address_id")

	var address userModels.Address
	if err:=database.DB.Where("id = ? AND user_id = ?",addressID,userID).First(&address).Error;err!=nil{
		helper.ResponseWithErr(c, http.StatusNotFound, "Address not found", "Invalid address ID", "")
        return
	}
	
	if err:=database.DB.Delete(&address).Error;err!=nil{
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to delete address", "Database error", "")
        return
	}

	c.JSON(http.StatusOK, gin.H{
        "status":  "ok",
        "message": "Address deleted successfully",
    })
}

func GetAddress(c *gin.Context) {
    userID, _ := c.Get("id")
    addressID := c.Param("address_id")

    var address userModels.Address
    if err := database.DB.Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error; err != nil {
        helper.ResponseWithErr(c, http.StatusNotFound, "Address not found", "Invalid address ID", "")
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "status":  "ok",
        "message": "Address fetched successfully",
        "address": address,
    })
}