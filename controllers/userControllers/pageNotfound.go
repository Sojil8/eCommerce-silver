package controllers

import (
	"net/http"

	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NotFound(c *gin.Context) {
	userID, exists := c.Get("id")
	logFields := []zap.Field{
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
	}
	if exists {
		logFields = append(logFields, zap.Any("user_id", userID))
	}

	pkg.Log.Warn("Page not found", logFields...)

	c.HTML(http.StatusNotFound, "pageNotfound.html", nil)
}