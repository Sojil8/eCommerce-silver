package middleware

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/helper"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

var SecretKey []byte

type Claims struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.StandardClaims
}

func SecretKeyCheck() {
	secret := os.Getenv("SECRET_KEY")
	if len(secret) == 0 {
		log.Fatal("--Failed to get secret key--")
	}
	SecretKey = []byte(secret)
	fmt.Println("--------------------SecretKey ok-------------------")
}

func GenerateToken(c *gin.Context, id int, email string, role string) (string, error) {
	claims := Claims{
		ID:    uint(id),
		Email: email,
		Role:  role,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 24).Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(SecretKey)
	if err != nil {
		return "Cant create token", err
	}
	return tokenString, nil
}

func Authenticate(cookieName, expectedRole, loginRedirect string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(cookieName)
		if err != nil {
			fmt.Printf("No %s cookie found: %v\n", cookieName, err)
			c.Redirect(http.StatusSeeOther, loginRedirect)
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(cookie, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return SecretKey, nil
		})
		if err != nil || !token.Valid {
			fmt.Println("Token invalid or parsing error:", err)
			c.Redirect(http.StatusSeeOther, loginRedirect)
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok || claims.Role != expectedRole {
			fmt.Printf("Claims issue - Role: %s, Expected: %s, OK: %v\n", claims.Role, expectedRole, ok)
			c.Redirect(http.StatusSeeOther, loginRedirect)
			c.Abort()
			return
		}

		if expectedRole == "Admin" {
			var admin adminModels.Admin
			if err := database.DB.First(&admin, claims.ID).Error; err != nil {
				fmt.Println("Admin not found in DB:", err)
				c.Redirect(http.StatusSeeOther, loginRedirect)
				c.Abort()
				return
			}
			c.Set("admin_id", claims.ID)
		} else if expectedRole == "User" {
			var user userModels.Users
			if err := database.DB.First(&user, claims.ID).Error; err != nil {
				fmt.Println("User not found in DB - ID:", claims.ID, "Error:", err)
				c.Redirect(http.StatusSeeOther, loginRedirect)
				c.Abort()
				return
			}
			if user.Is_blocked {
				helper.ResponseWithErr(c, http.StatusForbidden, "Account blocked", "Your account has been blocked", "")
				c.Abort()
				return
			}
			c.Set("user_name", user.UserName)
			c.Set("user", user)
		} else {
			c.Redirect(http.StatusSeeOther, loginRedirect)
			c.Abort()
			return
		}

		c.Set("id", claims.ID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Next()
	}
}
