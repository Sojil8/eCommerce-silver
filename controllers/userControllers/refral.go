package controllers

import (
	"net/http"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetInviteLink(c *gin.Context) {
	//get user
	uid, _ := c.Get("id")

	var user userModels.Users
	if err := database.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"Status": "Faild to get User",
		})
		return
	}

	//check if he had a invite link
	// if not create it
	if user.ReferralToken == "" {
		user.ReferralToken = uuid.New().String()
		//save the refral string to db
		if err := database.DB.Save(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"Status": "Faild to save User",
			})
			return
		}
	}

	inviteUrl := "http://localhost:3000/signup?ref=" + user.ReferralToken

	//give status code 200 and the user also
	c.JSON(http.StatusOK, gin.H{
		"invite_link": inviteUrl,
	})

}
