package controllers

import (
	"fmt"
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func ShowRefralPage(c *gin.Context) {
	pkg.Log.Info("Starting referral page retrieval")

	userID, exists := c.Get("id")
	if !exists {
		pkg.Log.Warn("User ID not found in context")
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User not authenticated", "User ID not found in context", "")
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		pkg.Log.Error("User not found",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "User referral code not found", err.Error())
		return
	}

	pkg.Log.Debug("Fetched user data",
		zap.Uint("user_id", userIDUint),
		zap.String("referral_token", user.ReferralToken))

	const baseUrl = "http://localhost:8888"
	refralLink := fmt.Sprintf("%s/signup?ref=%s", baseUrl, user.ReferralToken)
	pkg.Log.Debug("Generated referral link",
		zap.Uint("user_id", userIDUint),
		zap.String("referral_link", refralLink))

	var referrals []userModels.Refral
	if err := database.DB.Where("refred_user_id = ?", userID).Find(&referrals).Error; err != nil {
		pkg.Log.Error("Failed to fetch referrals",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch referrals", "Database error", err.Error())
		return
	}

	totalReferrals := len(referrals)
	var totalRewards uint
	var activeReferrals int
	for _, referral := range referrals {
		totalRewards += referral.RewardUser
		if referral.ReferralIsUsed {
			activeReferrals++
		}
	}

	pkg.Log.Info("Rendering referral page",
		zap.Uint("user_id", userIDUint),
		zap.String("referral_link", refralLink),
		zap.Int("total_referrals", totalReferrals),
		zap.Uint("total_rewards", totalRewards),
		zap.Int("active_referrals", activeReferrals))

	c.HTML(http.StatusOK, "refral.html", gin.H{
		"userCode":        refralLink,
		"totalReferrals":  totalReferrals,
		"totalRewards":    totalRewards,
		"activeReferrals": activeReferrals,
		"referrals":       referrals,
	})
}

func GetReferralData(c *gin.Context) {
	pkg.Log.Info("Starting referral data retrieval")

	userID, exists := c.Get("id")
	if !exists {
		pkg.Log.Warn("User ID not found in context")
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User not authenticated", "User ID not found in context", "")
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		pkg.Log.Error("Invalid user ID type",
			zap.Any("user_id", userID))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Invalid user ID type", "Error processing user ID", "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		pkg.Log.Error("User not found",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "User referral code not found", err.Error())
		return
	}

	pkg.Log.Debug("Fetched user data",
		zap.Uint("user_id", userIDUint),
		zap.String("referral_token", user.ReferralToken))

	var referrals []userModels.Refral
	if err := database.DB.Where("user_id = ?", userID).Preload("RefredUser").Find(&referrals).Error; err != nil {
		pkg.Log.Error("Failed to fetch referrals",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to fetch referrals", "Database error", err.Error())
		return
	}

	type ReferralResponse struct {
		FriendName string `json:"friendName"`
		Status     bool   `json:"status"`
		Reward     uint   `json:"reward"`
		Date       string `json:"date"`
	}

	var referralData []ReferralResponse
	totalReferrals := len(referrals)
	var totalRewards uint
	var activeReferrals int

	for _, referral := range referrals {
		var friend userModels.Users
		if err := database.DB.Where("id = ?", referral.RefredUserID).First(&friend).Error; err != nil {
			pkg.Log.Warn("Referred user not found",
				zap.Uint("user_id", userIDUint),
				zap.Uint("friend_id", referral.RefredUserID),
				zap.Error(err))
			continue
		}
		referralData = append(referralData, ReferralResponse{
			FriendName: friend.FirstName,
			Status:     referral.ReferralIsUsed,
			Reward:     referral.RewardUser,
			Date:       referral.CreatedAt.Format("2006-01-02"),
		})
		totalRewards += referral.RewardUser
		if referral.ReferralIsUsed {
			activeReferrals++
		}
	}

	pkg.Log.Info("Returning referral data",
		zap.Uint("user_id", userIDUint),
		zap.Int("total_referrals", totalReferrals),
		zap.Uint("total_rewards", totalRewards),
		zap.Int("active_referrals", activeReferrals),
		zap.Int("referral_data_count", len(referralData)))

	c.JSON(http.StatusOK, gin.H{
		"totalReferrals":  totalReferrals,
		"totalRewards":    totalRewards,
		"activeReferrals": activeReferrals,
		"referrals":       referralData,
	})
}