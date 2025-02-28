package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware(c *gin.Context) {
	session := sessions.Default(c)
	adminID := session.Get("admin_id")
	if adminID == nil {
		c.Redirect(http.StatusSeeOther, "/admin/login")
		c.Abort()
		return
	}
	c.Next()
}
