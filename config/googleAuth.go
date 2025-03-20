package config

import (
    "context"
    "encoding/json"
    "net/http"
    "os"

    "github.com/Sojil8/eCommerce-silver/database"
    "github.com/Sojil8/eCommerce-silver/helper"
    "github.com/Sojil8/eCommerce-silver/middleware"
    "github.com/Sojil8/eCommerce-silver/models/userModels"
    "github.com/gin-gonic/gin"
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
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
    url := GoogleOauthConfig.AuthCodeURL("state", oauth2.AccessTypeOffline)
    c.Redirect(http.StatusTemporaryRedirect, url)
}

func GoogleCallback(c *gin.Context) {
    code := c.Query("code")
    if code == "" {
        helper.ResponseWithErr(c, http.StatusBadRequest, "Code not found", "Invalid Google callback", "")
        return
    }

    token, err := GoogleOauthConfig.Exchange(context.Background(), code)
    if err != nil {
        helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to exchange token", err.Error(), "")
        return
    }

    client := GoogleOauthConfig.Client(context.Background(), token)
    resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
    if err != nil {
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
        helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to parse user info", err.Error(), "")
        return
    }

    var user userModels.Users
    err = database.DB.Where("email = ?", googleUser.Email).First(&user).Error
    if err != nil {
        if err.Error() == "record not found" {
            user = userModels.Users{
                UserName:   googleUser.Name,
                Email:      googleUser.Email,
                Password:   "", 
                Phone:      "",
                Is_blocked: false,
            }
            if err := database.DB.Create(&user).Error; err != nil {
                helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to create user", err.Error(), "")
                return
            }
        } else {
            helper.ResponseWithErr(c, http.StatusInternalServerError, "Failed to query user", err.Error(), "")
            return
        }
    }

    if user.Is_blocked {
        helper.ResponseWithErr(c, http.StatusForbidden, "Account blocked", "Your account has been blocked", "")
        return
    }

    jwtToken, err := middleware.GenerateToken(c, int(user.ID), user.Email, "User")
    if err != nil {
        helper.ResponseWithErr(c, http.StatusInternalServerError, "Token generation failed", err.Error(), "")
        return
    }

    c.SetCookie("jwt_token", jwtToken, 24*60*60, "/", "", false, true)
    c.Redirect(http.StatusSeeOther, "/home")
}