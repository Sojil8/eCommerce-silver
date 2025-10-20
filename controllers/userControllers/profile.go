package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/services"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/Sojil8/eCommerce-silver/utils/storage"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func ShowProfile(c *gin.Context) {
	pkg.Log.Info("Starting profile retrieval")

	userID, exists := c.Get("id")
	if !exists {
		pkg.Log.Warn("User ID not found in context")
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User ID not found in context", "Please log in", "/login")
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		pkg.Log.Warn("User data or username not found in context",
			zap.Uint("user_id", userIDUint))
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User data not found", "Please log in", "/login")
		return
	}

	userData := user.(userModels.Users)
	userNameStr := userName.(string)

	pkg.Log.Debug("Fetched user data",
		zap.Uint("user_id", userIDUint),
		zap.String("user_name", userNameStr))

	var wallet userModels.Wallet
	if err := database.DB.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
		pkg.Log.Debug("Wallet not found, creating new wallet",
			zap.Uint("user_id", userIDUint))
		wallet = userModels.Wallet{
			UserID:  userIDUint,
			Balance: 0.0,
		}
		if err := database.DB.Create(&wallet).Error; err != nil {
			pkg.Log.Error("Failed to create wallet",
				zap.Uint("user_id", userIDUint),
				zap.Error(err))
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create wallet", "Error creating wallet", "")
			return
		}
	}

	var addresses []userModels.Address
	if err := database.DB.Where("user_id = ?", userIDUint).Find(&addresses).Error; err != nil {
		pkg.Log.Error("Failed to fetch addresses",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		addresses = []userModels.Address{}
	}

	var orders []userModels.Orders
	if err := database.DB.Where("user_id = ?", userIDUint).Preload("OrderItems.Product").Order("order_date DESC").Find(&orders).Error; err != nil {
		pkg.Log.Error("Failed to fetch orders",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		orders = []userModels.Orders{}
	}

	var wishlistCount, cartCount int64
	if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
		pkg.Log.Warn("Failed to fetch wishlist count",
			zap.Uint("user_id", userData.ID),
			zap.Error(err))
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		pkg.Log.Warn("Failed to fetch cart count",
			zap.Uint("user_id", userData.ID),
			zap.Error(err))
		cartCount = 0
	}

	pkg.Log.Info("Rendering profile page",
		zap.Uint("user_id", userIDUint),
		zap.String("user_name", userNameStr),
		zap.Float64("wallet_balance", wallet.Balance),
		zap.Int("address_count", len(addresses)),
		zap.Int("order_count", len(orders)),
		zap.Int64("wishlist_count", wishlistCount),
		zap.Int64("cart_count", cartCount))

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
	pkg.Log.Info("Starting edit profile page retrieval")

	userID, _ := c.Get("id")
	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		pkg.Log.Error("User not found",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Invalid user ID", "")
		return
	}

	pkg.Log.Debug("Fetched user for edit profile",
		zap.Uint("user_id", userIDUint),
		zap.String("user_name", user.UserName))

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
	pkg.Log.Info("Starting profile update process")

	userID, _ := c.Get("id")
	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		pkg.Log.Error("User not found",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Profile not available", "")
		return
	}

	if err := c.ShouldBind(&editProfileRequest); err != nil {
		pkg.Log.Error("Failed to bind profile update request",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please check all fields", "")
		return
	}

	pkg.Log.Debug("Processing profile update request",
		zap.Uint("user_id", userIDUint),
		zap.String("username", editProfileRequest.UserName),
		zap.String("email", editProfileRequest.Email))

	var imgURL string
	file, header, err := c.Request.FormFile("profile_img")
	if err == nil {
		defer file.Close()
		imgURL, err = helper.ProcessImage(c, file, header)
		if err != nil {
			pkg.Log.Error("Failed to upload profile image",
				zap.Uint("user_id", userIDUint),
				zap.Error(err))
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to upload image", err.Error(), "")
			return
		}
		pkg.Log.Debug("Profile image uploaded",
			zap.Uint("user_id", userIDUint),
			zap.String("image_url", imgURL))
	}

	var existingUser userModels.Users
	if err := database.DB.Where("email = ? AND id != ?", editProfileRequest.Email, userID).First(&existingUser).Error; err == nil {
		pkg.Log.Warn("Email already exists",
			zap.Uint("user_id", userIDUint),
			zap.String("email", editProfileRequest.Email))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Email already exists", "Email already exists", "")
		return
	}

	if editProfileRequest.Email != user.Email {
		otp, err := helper.GenerateAndStoreOTP(editProfileRequest.Email)
		if err != nil {
			pkg.Log.Error("Failed to generate OTP",
				zap.Uint("user_id", userIDUint),
				zap.String("email", editProfileRequest.Email),
				zap.Error(err))
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
		if err := storage.RedisClient.Set(storage.Ctx, key, data, 15*time.Minute).Err(); err != nil {
			pkg.Log.Error("Failed to store OTP in Redis",
				zap.Uint("user_id", userIDUint),
				zap.String("redis_key", key),
				zap.Error(err))
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to store OTP", "Internal error", "")
			return
		}

		if err := services.SendOTP(editProfileRequest.Email, otp); err != nil {
			pkg.Log.Error("Failed to send OTP",
				zap.Uint("user_id", userIDUint),
				zap.String("email", editProfileRequest.Email),
				zap.Error(err))
			storage.RedisClient.Del(storage.Ctx, key)
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to send OTP", "Email sending failed", "")
			return
		}

		pkg.Log.Info("OTP sent for email verification",
			zap.Uint("user_id", userIDUint),
			zap.String("email", editProfileRequest.Email))

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
	user.FirstName = editProfileRequest.FirstName
	user.LastName = editProfileRequest.LastName
	user.Phone = editProfileRequest.Phone

	if err := database.DB.Save(&user).Error; err != nil {
		pkg.Log.Error("Failed to update profile",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update profile", "Database error", "")
		return
	}

	pkg.Log.Info("Profile updated successfully",
		zap.Uint("user_id", userIDUint),
		zap.String("user_name", user.UserName))

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Profile updated successfully",
	})
}

func ShowVerifyEditEmail(c *gin.Context) {
	pkg.Log.Info("Starting email verification page retrieval")

	userID, _ := c.Get("id")
	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Error": "Invalid user ID type",
		})
		return
	}

	email := c.Query("email")
	pkg.Log.Debug("Rendering email verification page",
		zap.Uint("user_id", userIDUint),
		zap.String("email", email))

	c.HTML(http.StatusOK, "verifyEmail.html", gin.H{
		"Email": email,
	})
}

var verifyEditEmailRequest struct {
	Email string `form:"email" binding:"required,email"`
	OTP   string `form:"otp" binding:"required,len=6"`
}

func VerifyEditEmail(c *gin.Context) {
	pkg.Log.Info("Starting email verification process")

	userID, _ := c.Get("id")
	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	if err := c.ShouldBind(&verifyEditEmailRequest); err != nil {
		pkg.Log.Error("Failed to bind email verification request",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please provide a valid OTP", "")
		return
	}

	pkg.Log.Debug("Processing email verification",
		zap.Uint("user_id", userIDUint),
		zap.String("email", verifyEditEmailRequest.Email))

	key := fmt.Sprintf("edit:%d", userID)
	data, err := storage.RedisClient.Get(storage.Ctx, key).Result()
	if err != nil {
		pkg.Log.Warn("OTP expired or not found",
			zap.Uint("user_id", userIDUint),
			zap.String("redis_key", key),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "OTP expired or not found", "Session expired", "")
		return
	}

	var tempData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &tempData); err != nil {
		pkg.Log.Error("Failed to unmarshal Redis data",
			zap.Uint("user_id", userIDUint),
			zap.String("redis_key", key),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to process OTP data", "Internal error", "")
		return
	}

	if tempData["otp"] != verifyEditEmailRequest.OTP {
		pkg.Log.Warn("Invalid OTP provided",
			zap.Uint("user_id", userIDUint),
			zap.String("email", verifyEditEmailRequest.Email))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid OTP", "OTP does not match", "")
		return
	}

	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		pkg.Log.Error("User not found",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Profile not available", "")
		return
	}

	user.UserName = tempData["username"].(string)
	user.FirstName = tempData["first_name"].(string)
	user.LastName = tempData["last_name"].(string)
	user.Email = tempData["email"].(string)
	user.Phone = tempData["phone"].(string)

	if imgURL, ok := tempData["profile_img"].(string); ok && imgURL != "" {
		user.ProfileImage = imgURL
	}

	if err := database.DB.Save(&user).Error; err != nil {
		pkg.Log.Error("Failed to update profile after OTP verification",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update profile", "Database error", "")
		return
	}

	if err := storage.RedisClient.Del(storage.Ctx, key).Err(); err != nil {
		pkg.Log.Warn("Failed to delete Redis OTP data",
			zap.Uint("user_id", userIDUint),
			zap.String("redis_key", key),
			zap.Error(err))
	}

	pkg.Log.Info("Email verification completed and profile updated",
		zap.Uint("user_id", userIDUint),
		zap.String("email", user.Email))

	c.Redirect(http.StatusSeeOther, "/profile")
}

var changePasswordRequest struct {
	CurrentPassword string `form:"current_password" binding:"required"`
	NewPassword     string `form:"new_password" binding:"required"`
	ConfirmPassword string `form:"confirm_password" binding:"required"`
}

func ShowChangePassword(c *gin.Context) {
	pkg.Log.Info("Starting change password page retrieval")

	userID, _ := c.Get("id")
	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Error": "Invalid user ID type",
		})
		return
	}

	pkg.Log.Debug("Rendering change password page",
		zap.Uint("user_id", userIDUint))

	c.HTML(http.StatusOK, "changePassProfile.html", gin.H{
		"status": "ok",
	})
}

func ChangePassword(c *gin.Context) {
	pkg.Log.Info("Starting password change process")

	userID, _ := c.Get("id")
	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	var user userModels.Users
	if err := database.DB.First(&user, userID).Error; err != nil {
		pkg.Log.Error("User not found",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "Profile not available", "")
		return
	}

	if err := c.ShouldBind(&changePasswordRequest); err != nil {
		pkg.Log.Error("Failed to bind password change request",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", "Please check all fields", "")
		return
	}

	if user.Password == "" {
		pkg.Log.Warn("No password set for user",
			zap.Uint("user_id", userIDUint))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "No password", "Password changes are only available for accounts created with email and password.", "")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(changePasswordRequest.NewPassword)); err == nil {
		pkg.Log.Warn("New password matches current password",
			zap.Uint("user_id", userIDUint))
		helper.ResponseWithErr(c, http.StatusUnauthorized, "Can't put original password as new password", "Can't put original password as new password", "")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(changePasswordRequest.CurrentPassword)); err != nil {
		pkg.Log.Warn("Incorrect current password",
			zap.Uint("user_id", userIDUint))
		helper.ResponseWithErr(c, http.StatusUnauthorized, "Incorrect current password", "Password mismatch", "")
		return
	}

	if changePasswordRequest.NewPassword != changePasswordRequest.ConfirmPassword {
		pkg.Log.Warn("New passwords do not match",
			zap.Uint("user_id", userIDUint))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Passwords do not match", "Please ensure new passwords match", "")
		return
	}

	hashedPassword, err := helper.HashPassword(changePasswordRequest.NewPassword)
	if err != nil {
		pkg.Log.Error("Failed to hash password",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to hash password", "Internal error", "")
		return
	}

	user.Password = hashedPassword
	if err := database.DB.Save(&user).Error; err != nil {
		pkg.Log.Error("Failed to update password",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update password", "Database error", "")
		return
	}

	pkg.Log.Info("Password changed successfully",
		zap.Uint("user_id", userIDUint))

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Password changed successfully",
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
	pkg.Log.Info("Starting address profile retrieval")

	userID, _ := c.Get("id")
	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	var addresses []userModels.Address
	if err := database.DB.Where("user_id = ?", userIDUint).Find(&addresses).Error; err != nil {
		pkg.Log.Error("Failed to fetch addresses",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Not found the address", err.Error(), "")
		return
	}

	pkg.Log.Info("Rendering address profile page",
		zap.Uint("user_id", userIDUint),
		zap.Int("address_count", len(addresses)))

	c.HTML(http.StatusOK, "addressProfile.html", gin.H{
		"status":    "ok",
		"Addresses": addresses,
	})
}

func AddAddress(c *gin.Context) {
	pkg.Log.Info("Starting address addition process")

	userID, _ := c.Get("id")
	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	if err := c.ShouldBindJSON(&addressRequest); err != nil {
		pkg.Log.Error("Failed to bind address request",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", err.Error(), "")
		return
	}

	pkg.Log.Debug("Processing address addition",
		zap.Uint("user_id", userIDUint),
		zap.String("address_type", addressRequest.AddressType),
		zap.String("city", addressRequest.City))

	address := userModels.Address{
		UserID:         userIDUint,
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
		pkg.Log.Error("Failed to add address",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to add address", "Database error", err.Error())
		return
	}

	pkg.Log.Info("Address added successfully",
		zap.Uint("user_id", userIDUint),
		zap.Uint("address_id", address.ID))

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Address added successfully",
		"address": address,
	})
}

func EditAddress(c *gin.Context) {
	pkg.Log.Info("Starting address update process")

	userID, _ := c.Get("id")
	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	addressID := c.Param("address_id")
	if err := c.ShouldBindJSON(&addressRequest); err != nil {
		pkg.Log.Error("Failed to bind address update request",
			zap.Uint("user_id", userIDUint),
			zap.String("address_id", addressID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusBadRequest, "Invalid input", err.Error(), "")
		return
	}

	var address userModels.Address
	if err := database.DB.Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error; err != nil {
		pkg.Log.Error("Address not found",
			zap.Uint("user_id", userIDUint),
			zap.String("address_id", addressID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Address not found", "Invalid address ID", "")
		return
	}

	pkg.Log.Debug("Processing address update",
		zap.Uint("user_id", userIDUint),
		zap.String("address_id", addressID),
		zap.String("address_type", addressRequest.AddressType),
		zap.String("city", addressRequest.City))

	address.AddressType = addressRequest.AddressType
	address.Name = addressRequest.Name
	address.City = addressRequest.City
	address.Landmark = addressRequest.Landmark
	address.State = addressRequest.State
	address.Pincode = addressRequest.Pincode
	address.Phone = addressRequest.Phone
	address.AlternatePhone = addressRequest.AlternatePhone

	if err := database.DB.Save(&address).Error; err != nil {
		pkg.Log.Error("Failed to update address",
			zap.Uint("user_id", userIDUint),
			zap.String("address_id", addressID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to update address", "Database error", err.Error())
		return
	}

	pkg.Log.Info("Address updated successfully",
		zap.Uint("user_id", userIDUint),
		zap.String("address_id", addressID))

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Address updated successfully",
		"address": address,
	})
}

func DeleteAddress(c *gin.Context) {
	pkg.Log.Info("Starting address deletion process")

	userID, _ := c.Get("id")
	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	addressID := c.Param("address_id")
	var address userModels.Address
	if err := database.DB.Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error; err != nil {
		pkg.Log.Error("Address not found",
			zap.Uint("user_id", userIDUint),
			zap.String("address_id", addressID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Address not found", "Invalid address ID", "")
		return
	}

	if err := database.DB.Delete(&address).Error; err != nil {
		pkg.Log.Error("Failed to delete address",
			zap.Uint("user_id", userIDUint),
			zap.String("address_id", addressID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to delete address", "Database error", "")
		return
	}

	pkg.Log.Info("Address deleted successfully",
		zap.Uint("user_id", userIDUint),
		zap.String("address_id", addressID))

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Address deleted successfully",
	})
}

func GetEditAddress(c *gin.Context) {
	pkg.Log.Info("Starting edit address retrieval")

	userID, _ := c.Get("id")
	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	addressID := c.Param("address_id")
	var address userModels.Address
	if err := database.DB.Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error; err != nil {
		pkg.Log.Error("Address not found",
			zap.Uint("user_id", userIDUint),
			zap.String("address_id", addressID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "Address not found", "Invalid address ID", "")
		return
	}

	pkg.Log.Info("Address fetched for editing",
		zap.Uint("user_id", userIDUint),
		zap.String("address_id", addressID),
		zap.String("address_type", address.AddressType))

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Address fetched successfully",
		"address": address,
	})
}

func ShowWallet(c *gin.Context) {
	pkg.Log.Info("Starting wallet page retrieval")

	userID := helper.FetchUserID(c)
	

	var wallet userModels.Wallet
	if err := database.DB.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			pkg.Log.Debug("Wallet not found, creating new wallet",
				zap.Uint("user_id", userID))
			wallet = userModels.Wallet{
				UserID:  userID,
				Balance: 0,
			}
			if err := database.DB.Create(&wallet).Error; err != nil {
				pkg.Log.Error("Failed to create wallet",
					zap.Uint("user_id", userID),
					zap.Error(err))
				helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to initialize wallet", "Error creating wallet", "")
				return
			}
		} else {
			pkg.Log.Error("Failed to fetch wallet",
				zap.Uint("user_id", userID),
				zap.Error(err))
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch wallet", err.Error(), "")
			return
		}
	}

	var transactionHistory []userModels.WalletTransaction
	if err := database.DB.Where("wallet_id = ?", wallet.ID).Order("created_at DESC").Find(&transactionHistory).Error; err != nil {
		pkg.Log.Error("Failed to fetch wallet transactions",
			zap.Uint("user_id", userID),
			zap.Uint("wallet_id", wallet.ID),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch transaction history", err.Error(), "")
		return
	}

	user, exists := c.Get("user")
	userName, nameExists := c.Get("user_name")
	if !exists || !nameExists {
		pkg.Log.Warn("User data or username not found in context",
			zap.Uint("user_id", userID))
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User data not found", "Please log in", "/login")
		return
	}

	userData := user.(userModels.Users)
	userNameStr := userName.(string)

	var wishlistCount, cartCount int64
	if err := database.DB.Model(&userModels.Wishlist{}).Where("user_id = ?", userData.ID).Count(&wishlistCount).Error; err != nil {
		pkg.Log.Warn("Failed to fetch wishlist count",
			zap.Uint("user_id", userData.ID),
			zap.Error(err))
		wishlistCount = 0
	}
	if err := database.DB.Model(&userModels.CartItem{}).Joins("JOIN carts ON carts.id = cart_items.cart_id").Where("carts.user_id = ?", userData.ID).Count(&cartCount).Error; err != nil {
		pkg.Log.Warn("Failed to fetch cart count",
			zap.Uint("user_id", userData.ID),
			zap.Error(err))
		cartCount = 0
	}

	pkg.Log.Info("Rendering wallet page",
		zap.Uint("user_id", userID),
		zap.String("user_name", userNameStr),
		zap.Uint("wallet_id", wallet.ID),
		zap.Float64("wallet_balance", wallet.Balance),
		zap.Int("transaction_count", len(transactionHistory)),
		zap.Int64("wishlist_count", wishlistCount),
		zap.Int64("cart_count", cartCount))

	c.HTML(http.StatusOK, "wallets.html", gin.H{
		"Wallet":        wallet,
		"WalletHistory": transactionHistory,
		"UserName":      userNameStr,
		"ProfileImage":  userData.ProfileImage,
		"WishlistCount": wishlistCount,
		"CartCount":     cartCount,
		"UserData":      userData,
		"ActiveTab":     "wallet",
	})
}
