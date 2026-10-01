// Package httpapi provides the HTTP delivery layer.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewHandler creates the API HTTP handler. Feature routes are registered as
// their application services are introduced.
func NewHandler() http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return router
}
