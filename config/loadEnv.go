package config

import (
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func LoadEnv() {
	if err := godotenv.Load(".env"); err != nil {
		pkg.Log.Fatal("Error loading .env file", zap.Error(err))
	}
	pkg.Log.Info("Environment file loaded successfully")
}