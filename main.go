package main

import (
	"github.com/Sojil8/eCommerce-silver/config"
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/routes"
	"github.com/Sojil8/eCommerce-silver/services"
	"github.com/Sojil8/eCommerce-silver/utils/helper"
	"github.com/Sojil8/eCommerce-silver/utils/storage"
	"github.com/gin-gonic/gin"
)

var router *gin.Engine

func init() {
	router = gin.Default()
	pkg.LoggerInit()
	router.Static("/static", "./static")
	router.SetFuncMap(config.SetupTemplateFunctions())
	router.LoadHTMLGlob("templates/**/*")
	config.LoadEnv()
	database.ConnectDb()
	services.Cloudinary()
	services.InitRazorPay()
	services.InitGoogleOAuth()
	middleware.SecretKeyCheck()
	storage.InitRedis()
	helper.StartCouponExpiryScheduler()

}

func main() {
	database.MigrageHandler()
	routes.AdminRoutes(router)
	routes.UserRoutes(router)

	router.Run()

}
