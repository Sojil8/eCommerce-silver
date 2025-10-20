package middleware

import (
	"net/http"
	"os"
	"time"

	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/models/userModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"go.uber.org/zap"
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
		pkg.Log.Fatal("Failed to get secret key from environment variable",
			zap.String("env_key", "SECRET_KEY"))
	}
	SecretKey = []byte(secret)
	pkg.Log.Info("Secret key initialized successfully")
}

func GenerateToken(c *gin.Context, id int, email string, role string) (string, error) {
	pkg.Log.Debug("Generating JWT token",
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
		zap.Int("id", id),
		zap.String("email", email),
		zap.String("role", role))

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
		pkg.Log.Error("Failed to create JWT token",
			zap.Error(err),
			zap.String("email", email),
			zap.String("role", role))
		return "", err
	}

	pkg.Log.Info("JWT token generated successfully",
		zap.String("email", email),
		zap.String("role", role))
	return tokenString, nil
}

func Authenticate(cookieName, expectedRole, loginRedirect string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Log middleware execution
		pkg.Log.Debug("Applying Authenticate middleware",
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.String("cookie_name", cookieName),
			zap.String("expected_role", expectedRole))

		// Check for cookie
		cookie, err := c.Cookie(cookieName)
		if err != nil {
			pkg.Log.Warn("No cookie found",
				zap.String("cookie_name", cookieName),
				zap.Error(err))
			c.Redirect(http.StatusSeeOther, loginRedirect)
			c.Abort()
			return
		}

		// Parse and validate JWT token
		token, err := jwt.ParseWithClaims(cookie, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return SecretKey, nil
		})
		if err != nil || !token.Valid {
			pkg.Log.Warn("Invalid or expired JWT token",
				zap.Error(err),
				zap.String("cookie_name", cookieName))
			c.Redirect(http.StatusSeeOther, loginRedirect)
			c.Abort()
			return
		}

		// Verify claims
		claims, ok := token.Claims.(*Claims)
		if !ok || claims.Role != expectedRole {
			pkg.Log.Warn("Invalid claims or role mismatch",
				zap.Bool("claims_ok", ok),
				zap.String("actual_role", claims.Role),
				zap.String("expected_role", expectedRole))
			c.Redirect(http.StatusSeeOther, loginRedirect)
			c.Abort()
			return
		}

		// Role-based database checks
		switch expectedRole {
		case "Admin":
			var admin adminModels.Admin
			if err := database.DB.First(&admin, claims.ID).Error; err != nil {
				pkg.Log.Warn("Admin not found in database",
					zap.Uint("id", claims.ID),
					zap.Error(err))
				c.Redirect(http.StatusSeeOther, loginRedirect)
				c.Abort()
				return
			}
			c.Set("admin_id", claims.ID)
			pkg.Log.Info("Admin authenticated",
				zap.Uint("admin_id", claims.ID),
				zap.String("email", claims.Email))
		case "User":
			var user userModels.Users
			if err := database.DB.First(&user, claims.ID).Error; err != nil {
				pkg.Log.Warn("User not found in database",
					zap.Uint("id", claims.ID),
					zap.Error(err))
				c.Redirect(http.StatusSeeOther, loginRedirect)
				c.Abort()
				return
			}
			if user.IsBlocked {
				pkg.Log.Warn("User account is blocked",
					zap.Uint("id", claims.ID),
					zap.String("user_name", user.UserName))
				c.SetCookie(cookieName, "", -1, "/", "", false, true)
				c.Redirect(http.StatusSeeOther, "/login?error=Your+account+has+been+blocked")
				c.Abort()
				return
			}
			c.Set("user_name", user.UserName)
			c.Set("user", user)
			pkg.Log.Info("User authenticated",
				zap.Uint("user_id", claims.ID),
				zap.String("user_name", user.UserName),
				zap.String("email", claims.Email))
		default:
			pkg.Log.Warn("Unsupported role",
				zap.String("role", expectedRole))
			c.Redirect(http.StatusSeeOther, loginRedirect)
			c.Abort()
			return
		}

		// Set context values
		c.Set("id", claims.ID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		pkg.Log.Debug("Authentication successful, proceeding to next handler",
			zap.Uint("id", claims.ID),
			zap.String("role", claims.Role))

		c.Next()
	}
}
