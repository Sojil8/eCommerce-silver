package helper

import (
	"github.com/Sojil8/eCommerce-silver/pkg"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	pkg.Log.Debug("Starting password hashing")

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		pkg.Log.Error("Failed to hash password",
			zap.Error(err))
		return "", err
	}

	pkg.Log.Debug("Password hashed successfully")
	return string(hashedBytes), nil
}