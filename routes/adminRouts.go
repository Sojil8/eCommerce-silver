package routes

import (
	controllers "github.com/Sojil8/eCommerce-silver/controllers/adminControllers"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/gin-gonic/gin"
)

func AdminRoutes(c *gin.Engine) {
	adminGroup := c.Group("/admin")
	{
		adminGroup.GET("/login", controllers.ShowLoginPage)
		adminGroup.POST("/login", controllers.AdminLogin)

		protected := adminGroup.Group("")
		protected.Use(middleware.AuthenticateAdmin())
		{

			adminGroup.GET("/user-management", controllers.GetUsers)
			adminGroup.PATCH("/block-user/:id", controllers.BlockUser)
			adminGroup.PATCH("/unblock-user/:id", controllers.UnBlockUser)
			adminGroup.DELETE("/delete-user/:id", controllers.DeleteUser)
			adminGroup.GET("/category", controllers.GetCategories)
			adminGroup.POST("/category/add", controllers.AddCategory)
			adminGroup.PATCH("/category/edit", controllers.EditCategory)
		}
	}

}
