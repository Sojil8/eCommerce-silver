package database

import (
	"github.com/Sojil8/eCommerce-silver/pkg"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"strings"

	"go.uber.org/zap"
)

var DB *gorm.DB

func maskDSN(dsn string) string {
	parts := strings.Split(dsn, " ")
	maskedParts := make([]string, len(parts))
	for i, part := range parts {
		if strings.HasPrefix(part, "password=") {
			maskedParts[i] = "password=****"
		} else if strings.HasPrefix(part, "user=") {
			maskedParts[i] = "user=****"
		} else {
			maskedParts[i] = part
		}
	}
	return strings.Join(maskedParts, " ")
}

func ConnectDb() {
	var err error
	dsn := os.Getenv("dataBase")
	if dsn == "" {
		pkg.Log.Warn("Database connection string (DSN) is missing in environment variables")
		return
	}

	maskedDSN := maskDSN(dsn)
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		pkg.Log.Fatal("Failed to connect to database",
			zap.String("dsn", maskedDSN),
			zap.Error(err))
	}

	pkg.Log.Info("Database connected successfully",
		zap.String("dsn", maskedDSN))
}

func GetDB() *gorm.DB {
	if DB == nil {
		pkg.Log.Debug("Database connection not initialized, attempting to reconnect")
		ConnectDb()
	}
	return DB
}