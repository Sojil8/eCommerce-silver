package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/services"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/Sojil8/eCommerce-silver/utils/storage"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func ShowProfile(c *gin.Context) {
	userID, exists := c.Get("id")
	if !exists {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User ID not found in context", "Please log in", "/login")
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User data not found", "Please log in", "/login")
		return
	}

	userData := user.(userModels.Users)
	userNameStr := userName.(string)

	
	var wallet userModels.Wallet
	if err := database.DB.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
		wallet = userModels.Wallet{
			UserID:  userIDUint,
			Balance: 0.0,
		}
		if err := database.DB.Create(&wallet).Error; err != nil {
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create wallet", "Error creating wallet", "")
			return
		}
	}

	var addresses []userModels.Address
	database.DB.Where("user_id = ?", userIDUint).Find(&addresses)

	var orders []userModels.Orders
	database.DB.Where("user_id = ?", userIDUint).Preload("OrderItems.Product").Order("order_date DESC").Find(&orders)

	var wishlistCount, cartCount int64
	database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount)
	database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount)

	c.HTML(http.StatusOK, "profileNew.html", gin.H{
		"User":          userData,
		"UserName":      userNameStr,
		"Wallet":        wallet,
		"Addresses":     addresses,
		"Orders":        orders,
		"ActiveTab":     "profile",
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
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
		storage.RedisClient.Set(storage.Ctx, key, data, 15*time.Minute)

		if err := services.SendOTP(editProfileRequest.Email, otp); err != nil {
			storage.RedisClient.Del(storage.Ctx, key)
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
	data, err := storage.RedisClient.Get(storage.Ctx, key).Result()
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
	storage.RedisClient.Del(storage.Ctx, key)
	c.Redirect(http.StatusSeeOther, "/profile")
}

var changePasswordRequest struct {
	CurrentPassword string `form:"current_password" binding:"required"`
	NewPassword     string `form:"new_password" binding:"required"`
	ConfirmPassword string `form:"confirm_password" binding:"required"`
}

func ShowChangePassword(c *gin.Context) {
	c.HTML(http.StatusOK, "changePassProfile.html", gin.H{
		"status": "ok",
	})
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

	if user.Password == "" {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "NO password", "Password changes are only available for accounts created with email and password.", "")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(changePasswordRequest.NewPassword)); err == nil {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "Can't put original password as new password", "Can't put original password as new password", "")
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

func GetAddressProfile(c *gin.Context) {
	uuid, _ := c.Get("id")

	var addressess []userModels.Address
	if err := database.DB.Where("user_id = ?", uuid).Find(&addressess).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Not found the address", err.Error(), "")
	}

	c.HTML(http.StatusOK, "addressProfile.html", gin.H{
		"status":    "ok",
		"Addresses": addressess,
	})
}

func AddAddress(c *gin.Context) {
	userID, _ := c.Get("id")

	if err := c.ShouldBindJSON(&addressRequest); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", err.Error(), "")
		return
	}

	address := userModels.Address{
		UserID:         userID.(uint),
		AddressType:    addressRequest.AddressType,
		Name:           addressRequest.Name,
		City:           addressRequest.City,
		Landmark:       addressRequest.Landmark,
		State:          addressRequest.State,
		Pincode:        addressRequest.Pincode,
		Phone:          addressRequest.Phone,
		AlternatePhone: addressRequest.AlternatePhone,
	}

	if err := database.DB.Create(&address).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to add address", "Database error", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Address added successfully",
		"address": address,
	})
}
func EditAddress(c *gin.Context) {
	userID, _ := c.Get("id")
	addressID := c.Param("address_id")

	if err := c.ShouldBindJSON(&addressRequest); err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", err.Error(), "")
		return
	}

	var address userModels.Address
	if err := database.DB.Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Address not found", "Invalid address ID", "")
		return
	}

	address.AddressType = addressRequest.AddressType
	address.Name = addressRequest.Name
	address.City = addressRequest.City
	address.Landmark = addressRequest.Landmark
	address.State = addressRequest.State
	address.Pincode = addressRequest.Pincode
	address.Phone = addressRequest.Phone
	address.AlternatePhone = addressRequest.AlternatePhone

	if err := database.DB.Save(&address).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update address", "Database error", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Address updated successfully",
		"address": address,
	})
}
func DeleteAddress(c *gin.Context) {
	userID, _ := c.Get("id")
	addressID := c.Param("address_id")

	var address userModels.Address
	if err := database.DB.Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "Address not found", "Invalid address ID", "")
		return
	}

	if err := database.DB.Delete(&address).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to delete address", "Database error", "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Address deleted successfully",
	})
}

func GetEditAddress(c *gin.Context) {
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

func ShowWallet(c *gin.Context) {
	userID, _ := c.Get("id")
	var wallet userModels.Wallet
	if err := database.DB.Where("user_id = ?", userID).First(&wallet).Error; err != nil {

		wallet = userModels.Wallet{
			UserID:  userID.(uint),
			Balance: 0,
		}
		if err := database.DB.Create(&wallet).Error; err != nil {
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to initialize wallet", "Error creating wallet", "")
			return
		}
	}
	var walletData userModels.WalletTransaction
	if err:=database.DB.Find(&walletData,wallet.ID).Error;err!=nil{
		helper.ResponseWithErr(c,http.StatusNotFound,"Transaction history not found","Transaction history not found","")
		return
	}

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User data not found", "Please log in", "/login")
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

	c.HTML(http.StatusOK, "wallets.html", gin.H{
		"Wallet": wallet,

		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
		"UserData":      userData,
		"ActiveTab":     "wallet",
		"WalletHistory":walletData,
	})
}
