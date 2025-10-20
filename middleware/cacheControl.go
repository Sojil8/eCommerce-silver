package middleware

import "github.com/gin-gonic/gin"

func ClearCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, no-transform, must-revalidate, private, max-age=0")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "Mon, 26 Jul 1997 05:00:00 GMT") 
		c.Header("X-Accel-Expires", "0")                     
		c.Next()
	}
}

func PreventBackButton() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Header("X-Prevent-Back", "true")
		c.Next()
	}
}
