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
		log.Fatal("--Faild to get secret key--")
	}
	SecretKey = []byte(secret)
	fmt.Println("--------------------SecretKey ok-------------------")

}

func CheckTokenCreation(c *gin.Context, userId uint, email, role string) {
	token, err := GenerateToken(c, userId, email, role)
	if err != nil {
		helper.ResponseWithErr(c, http.StatusBadRequest, "error in jwt claims", "Error creating jwt", "")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "OK",
		"token":  token,
		"code":   200,
	})
}

func GenerateToken(c *gin.Context, id uint, email string, role string) (string, error) {
	claims := Claims{
		ID:    id,
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

func AuthenticateAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("admin_token")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/admin/login")
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(cookie, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			c.Redirect(http.StatusSeeOther, "/admin/login")
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok || claims.Role != "admin" {
			c.Redirect(http.StatusSeeOther, "/admin/login")
			c.Abort()
			return
		}

		// Optional: Verify admin exists in DB
		var admin adminModels.Admin
		if err := database.DB.First(&admin, claims.Id).Error; err != nil {
			c.Redirect(http.StatusSeeOther, "/admin/login")
			c.Abort()
			return
		}

		c.Set("admin_id", claims.Id)
		c.Set("email", claims.Email)
		c.Next()
	}
}
