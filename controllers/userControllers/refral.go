package controllers

import (
	"fmt"
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
)

func ShowRefralPage(c *gin.Context) {
	userID, exists := c.Get("id")
	if !exists {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User not authenticated", "User ID not found in context", "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "User referral code not found", err.Error())
		return
	}

	const baseUrl = "http://localhost:8888"
	refralLink := fmt.Sprintf("%s/signup?ref=%s", baseUrl, user.ReferralToken)


	var referrals []userModels.Refral
	if err := database.DB.Where("refred_user_id = ?", userID).Find(&referrals).Error; err != nil {
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

	c.HTML(http.StatusOK, "refral.html", gin.H{
		"userCode":        refralLink,
		"totalReferrals":  totalReferrals,
		"totalRewards":    totalRewards,
		"activeReferrals": activeReferrals,
		"referrals":       referrals,
	})
}

func GetReferralData(c *gin.Context) {
	userID, exists := c.Get("id")
	if !exists {
		helper.ResponseWithErr(c, http.StatusUnauthorized, "User not authenticated", "User ID not found in context", "")
		return
	}

	var user userModels.Users
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		helper.ResponseWithErr(c, http.StatusNotFound, "User not found", "User referral code not found", err.Error())
		return
	}

	var referrals []userModels.Refral
	if err := database.DB.Where("user_id = ?", userID).Preload("RefredUser").Find(&referrals).Error; err != nil {
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

	c.JSON(http.StatusOK, gin.H{
		"totalReferrals":  totalReferrals,
		"totalRewards":    totalRewards,
		"activeReferrals": activeReferrals,
		"referrals":       referralData,
	})
}