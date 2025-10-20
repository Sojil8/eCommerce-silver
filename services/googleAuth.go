package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/Sojil8/eCommerce-silver/config"
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

var GoogleOauthConfig *oauth2.Config

func InitGoogleOAuth() {
	GoogleOauthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  "http://localhost:8888/auth/google/callback",
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func GoogleLogin(c *gin.Context) {
	referralCode := c.Query("ref")
	log.Printf("Referral code from google login: %s", referralCode)

	state := fmt.Sprintf("oauth_state_%s", referralCode)
	url := GoogleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		helper.ResponseWithErr(c, http.StatusBadRequest, "Authorization code not found", "Invalid Google callback - no code", "")
		return
	}

	var referralCode string
	if state != "" && strings.HasPrefix(state, "oauth_state_") {
		referralCode = strings.TrimPrefix(state, "oauth_state_")
	}
	log.Printf("Referral code from state: '%s'", referralCode)

	token, err := GoogleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		log.Printf("Token exchange failed: %v", err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to exchange token", err.Error(), "")
		return
	}

	client := GoogleOauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		log.Printf("Failed to get user info: %v", err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to get user info", err.Error(), "")
		return
	}
	defer resp.Body.Close()

	var googleUser struct {
		Email      string `json:"email"`
		Name       string `json:"name"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		log.Printf("Failed to parse user info: %v", err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to parse user info", err.Error(), "")
		return
	}

	var user userModels.Users
	err = database.DB.Where("email = ?", googleUser.Email).First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			err := database.DB.Transaction(func(tx *gorm.DB) error {
				userReferralCode, err := helper.GenerateReferralCode()
				if err != nil {
					return fmt.Errorf("referral code generation failed: %w", err)
				}

				user = userModels.Users{
					UserName:      googleUser.Name,
					Email:         googleUser.Email,
					Password:      "", 
					Phone:         "", 
					IsBlocked:    false,
					ReferralToken: userReferralCode,
				}

				if err := tx.Create(&user).Error; err != nil {
					return fmt.Errorf("could not create user: %w", err)
				}
				log.Printf("Created new user with ID: %d", user.ID)

				
				if referralCode != "" {
					log.Printf("Processing referral code: '%s' for user ID: %d", referralCode, user.ID)
					if err := config.VerifiRefralCode(user.ID, referralCode); err != nil {
						log.Printf("Referral verification failed for new OAuth user %d (code: %s): %v", user.ID, referralCode, err)
					} else {
						log.Printf("Referral verification successful for user ID: %d", user.ID)
					}
				} else {
					log.Printf("No referral code provided or invalid referral code")
				}

				return nil
			})

			if err != nil {
				log.Printf("User creation transaction failed: %v", err)
				helper.ResponseWithErr(c, http.StatusInternalServerError, "User creation failed", err.Error(), "")
				return
			}

			log.Printf("Successfully created user: %s with ID: %d", user.Email, user.ID)
		} else {
			log.Printf("Database query failed: %v", err)
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to query user", err.Error(), "")
			return
		}
	} else {
		log.Printf("Existing user found: %s with ID: %d", user.Email, user.ID)
	}

	if user.IsBlocked {
		c.Redirect(http.StatusFound, "/login?error=Your+account+has+been+blocked")
		return
	}

	jwtToken, err := middleware.GenerateToken(c, int(user.ID), user.Email, "User")
	if err != nil {
		log.Printf("Token generation failed: %v", err)
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Token generation failed", err.Error(), "")
		return
	}

	c.SetCookie("jwt_token", jwtToken, 24*60*60, "/", "", false, true)
	c.Redirect(http.StatusSeeOther, "/home")
}
