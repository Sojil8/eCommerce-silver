package config

import (
	"fmt"
	"log"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"gorm.io/gorm"
)

func VerifiRefralCode(newUserID uint, refralCode string) error {

	// newUserID, ok := id.(int)
	// if !ok {
	// 	return fmt.Errorf("can't convert user id in verify refral api")
	// }

	if refralCode == "" {
		return nil
	}

	return database.DB.Transaction(func(tx *gorm.DB) error {
		var currentUser userModels.Users
		if err := tx.Where("id = ?", newUserID).First(&currentUser).Error; err != nil {
			return fmt.Errorf("current user not found %s", err)
		}
		if currentUser.ReferralToken == refralCode {
			return fmt.Errorf("can't use user own code")
		}

		var existingRefral userModels.Refral
		if err := tx.Where("user_id = ?", newUserID).First(&existingRefral).Error; err == nil {
			return fmt.Errorf("user %d has already used a referral code", newUserID)
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("database error checking referral usage: %w", err)
		}

		var refredUser userModels.Users
		if err := tx.Where("referral_token = ?", refralCode).First(&refredUser).Error; err != nil {
			return fmt.Errorf("invalid Code %s", err)
		}

		if currentUser.CreatedAt.Before(refredUser.CreatedAt) || currentUser.CreatedAt.Equal(refredUser.CreatedAt) {
			return fmt.Errorf("you cannot use this referral code because the code owner joined after you")
		}

		wallet, err := EnshureWallet(tx, newUserID)
		if err != nil {
			return err
		}
		wallet.Balance += 200
		if err := tx.Save(&wallet).Error; err != nil {
			return fmt.Errorf("error giving reward to user")
		}
		log.Println("got enshure wallet*************")

		var invitePersonWallet userModels.Wallet
		if err := tx.Where("user_id = ?", refredUser.ID).First(&invitePersonWallet).Error; err != nil {
			return fmt.Errorf("user wallet not found")
		}
		invitePersonWallet.Balance += 100
		if err := tx.Save(&invitePersonWallet).Error; err != nil {
			return fmt.Errorf("error giving reward to user %s", err)
		}

		createUserRefral := userModels.Refral{
			UserID:             uint(newUserID),
			RefredUserID:       refredUser.ID,
			RewardUser:         200,
			RewardInvitePerson: 100,
			RefralCode:         refralCode,
			ReferralIsUsed:     true,
		}

		if err := tx.Create(&createUserRefral).Error; err != nil {
			return fmt.Errorf("refral data create error %s", err)
		}

		return nil
	})

}
