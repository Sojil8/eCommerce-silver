package userModels

import "gorm.io/gorm"

type Refral struct {
	gorm.Model
	UserID             uint   `json:"user_id"`
	RefredUserID       uint   `json:"refral_user_id"`
	RewardUser         uint   `json:"reward_for_user"`
	RewardInvitePerson uint   `json:"Reward_invite_person"`
	RefralCode         string `json:"refralcode"`
	ReferralIsUsed     bool   `json:"referral_is_used" gorm:"default:false"`
}
