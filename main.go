package main

import (
	"github.com/Sojil8/eCommerce-silver/config"
	"github.com/Sojil8/eCommerce-silver/database"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/Sojil8/eCommerce-silver/routes"
	"github.com/gin-gonic/gin"
)

var router *gin.Engine

func init() {
	router = gin.Default()
	router.Static("/static", "./static")
	router.LoadHTMLGlob("templates/**/*")
	config.LoadEnv()
	config.Cloudnary()
	database.ConnectDb()
	middleware.SecretKeyCheck()
	database.MigrageHandler()
	database.InitRedis()

}

func main() {
	routes.AdminRoutes(router)
	routes.UserRoutes(router)

	router.Run()
}
