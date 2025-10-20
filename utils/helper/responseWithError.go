package helper

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ResponseWithErr(c *gin.Context, status int, err, msg, redirect string) {

	 if redirect != "" && (status == http.StatusSeeOther || status == http.StatusFound || status == http.StatusMovedPermanently) {
        // Store error message in flash/session if needed
        c.Redirect(status, redirect)
        return
    }
	c.JSON(status, gin.H{
		"status":   http.StatusText(status),
		"error":    err,
		"message":  msg,
		"redirect": redirect,
	})
}
