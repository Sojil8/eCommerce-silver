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
	ID    int    `json:"id"`
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
	token, err := GenerateToken(c, int(userId), email, role)
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

func GenerateToken(c *gin.Context, id int, email string, role string) (string, error) {
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
		ClearCache()

		cookie, err := c.Cookie("jwtTokensAdmin")
		if err != nil {
			fmt.Println("No cookie found:", err)
			c.Redirect(http.StatusSeeOther, "/admin/login")
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(cookie, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(os.Getenv("SECRET_KEY")), nil
		})
		if err != nil || !token.Valid {
			fmt.Println("Token invalid or parsing error:", err)
			c.Redirect(http.StatusSeeOther, "/admin/login")
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok || claims.Role != "Admin" {
			fmt.Println("Claims issue - Role:", claims.Role, "OK:", ok)
			c.Redirect(http.StatusSeeOther, "/admin/login")
			c.Abort()
			return
		}

		var admin adminModels.Admin
		if err := database.DB.First(&admin, claims.ID).Error; err != nil {
			fmt.Println("Admin not found in DB:", err)
			c.Redirect(http.StatusSeeOther, "/admin/login")
			c.Abort()
			return
		}

		c.Set("admin_id", claims.ID)
		c.Set("email", claims.Email)
		// fmt.Println("Admin authenticated:", claims.Email)
		c.Next()
	}
}

// AuthenticateUser is a middleware to protect user routes
func AuthenticateUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		ClearCache()

		cookie, err := c.Cookie("jwt_token")
		if err != nil {
			fmt.Println("No user cookie found:", err)
			c.Redirect(http.StatusSeeOther, "/user/login")
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
			c.Redirect(http.StatusSeeOther, "/user/login")
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok || claims.Role != "User" {
			fmt.Println("Claims issue - Role:", claims.Role, "OK:", ok)
			c.Redirect(http.StatusSeeOther, "/user/login")
			c.Abort()
			return
		}

		var user userModels.User
		if err := database.DB.First(&user, claims.ID).Error; err != nil {
			fmt.Println("User not found in DB - ID:", claims.ID, "Error:", err)
			c.Redirect(http.StatusSeeOther, "/user/login")
			c.Abort()
			return
		}

		if user.Is_blocked {
			helper.ResponseWithErr(c, http.StatusForbidden, "Account blocked", "Your account has been blocked", "")
			c.Abort()
			return
		}

		c.Set("user_id", claims.ID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("user_name", user.UserName) // Ensure UserName is set
		fmt.Println("User authenticated - ID:", claims.ID, "Email:", claims.Email, "UserName:", user.UserName)
		c.Next()
	}
}
