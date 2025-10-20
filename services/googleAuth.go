package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/Sojil8/eCommerce-silver/config"
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

var GoogleOauthConfig *oauth2.Config

func InitGoogleOAuth() {
	pkg.Log.Debug("Initializing Google OAuth configuration")

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

	if GoogleOauthConfig.ClientID == "" || GoogleOauthConfig.ClientSecret == "" {
		pkg.Log.Fatal("Failed to initialize Google OAuth: missing client ID or secret",
			zap.String("clientID", GoogleOauthConfig.ClientID),
			zap.String("clientSecret", GoogleOauthConfig.ClientSecret))
	}

	pkg.Log.Info("Google OAuth configuration initialized successfully")
}

func GoogleLogin(c *gin.Context) {
	referralCode := c.Query("ref")
	pkg.Log.Debug("Initiating Google login",
		zap.String("referralCode", referralCode),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method))

	state := "oauth_state_" + referralCode
	url := GoogleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	pkg.Log.Info("Redirecting to Google OAuth URL",
		zap.String("state", state),
		zap.String("url", url))

	c.Redirect(http.StatusTemporaryRedirect, url)
}

func GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	pkg.Log.Debug("Processing Google OAuth callback",
		zap.String("code", code),
		zap.String("state", state),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method))

	if code == "" {
		pkg.Log.Warn("Authorization code not found in Google callback")
		helper.ResponseWithErr(c, http.StatusBadRequest, "Authorization code not found", "Invalid Google callback - no code", "")
		return
	}

	var referralCode string
	if state != "" && strings.HasPrefix(state, "oauth_state_") {
		referralCode = strings.TrimPrefix(state, "oauth_state_")
		pkg.Log.Debug("Extracted referral code from state",
			zap.String("referralCode", referralCode))
	} else {
		pkg.Log.Debug("No referral code in state",
			zap.String("state", state))
	}

	token, err := GoogleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		pkg.Log.Error("Failed to exchange OAuth token",
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to exchange token", err.Error(), "")
		return
	}
	pkg.Log.Debug("OAuth token exchanged successfully",
		zap.String("tokenType", token.TokenType))

	client := GoogleOauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		pkg.Log.Error("Failed to get user info from Google",
			zap.Error(err))
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
		pkg.Log.Error("Failed to parse Google user info",
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to parse user info", err.Error(), "")
		return
	}
	pkg.Log.Info("Retrieved Google user info",
		zap.String("email", googleUser.Email),
		zap.String("name", googleUser.Name))

	var user userModels.Users
	err = database.DB.Where("email = ?", googleUser.Email).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			pkg.Log.Debug("User not found, creating new user",
				zap.String("email", googleUser.Email))

			err := database.DB.Transaction(func(tx *gorm.DB) error {
				userReferralCode, err := helper.GenerateReferralCode()
				if err != nil {
					pkg.Log.Error("Failed to generate referral code",
						zap.String("email", googleUser.Email),
						zap.Error(err))
					return fmt.Errorf("referral code generation failed: %w", err)
				}

				user = userModels.Users{
					UserName:      googleUser.Name,
					Email:         googleUser.Email,
					Password:      "",
					Phone:         "",
					IsBlocked:     false,
					ReferralToken: userReferralCode,
				}

				if err := tx.Create(&user).Error; err != nil {
					pkg.Log.Error("Failed to create user",
						zap.String("email", googleUser.Email),
						zap.Error(err))
					return fmt.Errorf("could not create user: %w", err)
				}
				pkg.Log.Info("Created new user",
					zap.Uint("userID", user.ID),
					zap.String("email", user.Email),
					zap.String("referralToken", user.ReferralToken))

				if referralCode != "" {
					pkg.Log.Debug("Processing referral code for new user",
						zap.Uint("userID", user.ID),
						zap.String("referralCode", referralCode))
					if err := config.VerifiRefralCode(user.ID, referralCode); err != nil {
						pkg.Log.Warn("Referral verification failed",
							zap.Uint("userID", user.ID),
							zap.String("referralCode", referralCode),
							zap.Error(err))
					} else {
						pkg.Log.Info("Referral verification successful",
							zap.Uint("userID", user.ID),
							zap.String("referralCode", referralCode))
					}
				} else {
					pkg.Log.Debug("No referral code provided for new user",
						zap.Uint("userID", user.ID))
				}

				return nil
			})

			if err != nil {
				pkg.Log.Error("User creation transaction failed",
					zap.String("email", googleUser.Email),
					zap.Error(err))
				helper.ResponseWithErr(c, http.StatusInternalServerError, "User creation failed", err.Error(), "")
				return
			}
		} else {
			pkg.Log.Error("Failed to query user in database",
				zap.String("email", googleUser.Email),
				zap.Error(err))
			helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to query user", err.Error(), "")
			return
		}
	} else {
		pkg.Log.Info("Existing user found",
			zap.Uint("userID", user.ID),
			zap.String("email", user.Email))
	}

	if user.IsBlocked {
		pkg.Log.Warn("User account is blocked",
			zap.Uint("userID", user.ID),
			zap.String("email", user.Email))
		c.Redirect(http.StatusFound, "/login?error=Your+account+has+been+blocked")
		return
	}

	jwtToken, err := middleware.GenerateToken(c, int(user.ID), user.Email, "User")
	if err != nil {
		pkg.Log.Error("Failed to generate JWT token",
			zap.Uint("userID", user.ID),
			zap.String("email", user.Email),
			zap.Error(err))
		helper.ResponseWithErr(c, http.StatusInternalServerError, "Token generation failed", err.Error(), "")
		return
	}
	pkg.Log.Info("JWT token generated for user",
		zap.Uint("userID", user.ID),
		zap.String("email", user.Email))

	c.SetCookie("jwt_token", jwtToken, 24*60*60, "/", "", false, true)
	pkg.Log.Info("Redirecting to home after successful Google OAuth login",
		zap.Uint("userID", user.ID),
		zap.String("email", user.Email))
	c.Redirect(http.StatusSeeOther, "/home")
}
