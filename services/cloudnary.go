package services

import (
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"go.uber.org/zap"
)

func Cloudinary() {
	pkg.Log.Info("Starting Cloudinary initialization")

	if err := helper.InitCloudinary(); err != nil {
		pkg.Log.Fatal("Failed to initialize Cloudinary",
			zap.Error(err))
	}

	pkg.Log.Info("Cloudinary initialized successfully")
}