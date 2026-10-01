// Package httpapi provides the HTTP delivery layer.
package httpapi

import (
	"net/http"
	"strings"

	"github.com/chokosabe/hotel-bookings/internal/evaluatordata"
	"github.com/chokosabe/hotel-bookings/internal/hotels"
	"github.com/gin-gonic/gin"
)

// Dependencies are the application services used by HTTP handlers.
type Dependencies struct {
	Hotels              *hotels.Service
	TestData            *evaluatordata.Service
	EnableTestEndpoints bool
}

// NewHandler creates the API HTTP handler.
func NewHandler(deps Dependencies) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := router.Group("/api/v1")
	if deps.Hotels != nil {
		api.GET("/hotels", searchHotels(deps.Hotels))
		api.GET("/hotels/:hotelID/availability", availableRooms(deps.Hotels))
	}
	if deps.EnableTestEndpoints && deps.TestData != nil {
		test := api.Group("/test")
		test.POST("/reset", resetData(deps.TestData))
		test.POST("/seed", seedData(deps.TestData))
	}

	return router
}

func searchHotels(service *hotels.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.TrimSpace(c.Query("name"))
		if name == "" {
			writeError(c, http.StatusBadRequest, "invalid_request", "name query parameter is required")
			return
		}

		found, err := service.Search(c.Request.Context(), name)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", "unable to search hotels")
			return
		}
		c.JSON(http.StatusOK, found)
	}
}

func resetData(service *evaluatordata.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := service.Reset(c.Request.Context()); err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", "unable to reset test data")
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func seedData(service *evaluatordata.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := service.Seed(c.Request.Context()); err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", "unable to seed test data")
			return
		}
		c.Status(http.StatusNoContent)
	}
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorEnvelope{Error: apiError{Code: code, Message: message}})
}
