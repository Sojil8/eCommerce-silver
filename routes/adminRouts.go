package routes

import (
	controllers "github.com/Sojil8/eCommerce-silver/controllers/adminControllers"
	"github.com/Sojil8/eCommerce-silver/middleware"
	"github.com/gin-gonic/gin"
)

var roleAdmin = "Admin"

func AdminRoutes(c *gin.Engine) {
	adminGroup := c.Group("/admin")
	{
		adminGroup.GET("/login", controllers.ShowLoginPage)
		adminGroup.POST("/login", controllers.AdminLogin)

		protected := adminGroup.Group("")
		protected.Use(middleware.Authenticate("jwtTokensAdmin","Admin","/admin/login"),middleware.ClearCache())
		{
			//user-management route
			protected.GET("/user-management", controllers.GetUsers)
			protected.PATCH("/block-user/:id", controllers.BlockUser)
			protected.PATCH("/unblock-user/:id", controllers.UnBlockUser)

			// Category routes
			protected.GET("/category", controllers.GetCategories)
			protected.POST("/category/add", controllers.AddCategory)
			protected.PATCH("/category/edit/:id", controllers.EditCategory) 
			protected.PATCH("/category/list/:id", controllers.ListCategory)  
			protected.PATCH("/category/unlist/:id", controllers.UnlistCategory)

			//product roures
			protected.GET("/products", controllers.GetProducts)
			protected.GET("/products/add",controllers.ShowAddProductForm)
			protected.POST("/products/add", controllers.AddProduct)
			protected.GET("/products/edit/:id", controllers.ShowEditProductForm)
			protected.PATCH("/products/edit/:id", controllers.EditProduct)

			// protected.GET("/products/details/:id",controllers.ProductDetailsHandler)
			protected.PATCH("/products/toggle/:id", controllers.ToggleProductStatus)

			protected.POST("/logout", controllers.AdminLogout)
		}
	}
}
