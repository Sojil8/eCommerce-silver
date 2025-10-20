package helper

import (
	"net/http"

	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func ResponseWithErr(c *gin.Context, status int, err, msg, redirect string) {
	pkg.Log.Debug("Preparing response",
		zap.Int("status", status),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
		zap.String("redirect", redirect))

	if redirect != "" && (status == http.StatusSeeOther || status == http.StatusFound || status == http.StatusMovedPermanently) {
		pkg.Log.Info("Performing redirect",
			zap.Int("status", status),
			zap.String("redirect", redirect),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method))
		c.Redirect(status, redirect)
		return
	}

	response := gin.H{
		"status":   http.StatusText(status),
		"error":    err,
		"message":  msg,
		"redirect": redirect,
	}
	pkg.Log.Info("Sending JSON response",
		zap.Int("status", status),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
		zap.Any("response", response))
	c.JSON(status, response)
}