package main

import (
	"github.com/Sojil8/eCommerce-silver/config"
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/Sojil8/eCommerce-silver/routes"
	"github.com/Sojil8/eCommerce-silver/services"
	"github.com/Sojil8/eCommerce-silver/utils/storage"
	"github.com/gin-gonic/gin"
)

var router *gin.Engine

func init() {
	router = gin.Default()
	router.Static("/static", "./static")
	router.SetFuncMap(config.SetupTemplateFunctions())
	router.LoadHTMLGlob("templates/**/*")
	config.LoadEnv()
	services.Cloudnary()
	services.InitRazorPay()
	services.InitGoogleOAuth()
	database.ConnectDb()
	middleware.SecretKeyCheck()
	database.MigrageHandler()
	storage.InitRedis()
	pkg.LoggerInit()
}

func main() {
	routes.AdminRoutes(router)
	routes.UserRoutes(router)

	router.Run()

	
}
