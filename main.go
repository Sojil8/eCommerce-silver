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
	router.SetFuncMap(config.SetupTemplateFunctions())
	router.LoadHTMLGlob("templates/**/*")
	config.LoadEnv()
	config.Cloudnary()
	config.InitGoogleOAuth()
	database.ConnectDb()
	middleware.SecretKeyCheck()
	database.MigrageHandler()
	database.InitRedis()
	// fmt.Println("GOOGLE_CLIENT_ID:", os.Getenv("GOOGLE_CLIENT_ID"))
	// fmt.Println("GOOGLE_CLIENT_SECRET:", os.Getenv("GOOGLE_CLIENT_SECRET"))
}

func main() {
	routes.AdminRoutes(router)
	routes.UserRoutes(router)
	

	
	router.Run()
}
