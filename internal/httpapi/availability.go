package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/domain"
	"github.com/chokosabe/hotel-bookings/internal/hotels"
	"github.com/gin-gonic/gin"
)

const dateLayout = "2006-01-02"

type availableRoomResponse struct {
	ID       int64  `json:"id"`
	Number   string `json:"number"`
	Type     string `json:"type"`
	Capacity int    `json:"capacity"`
}

func availableRooms(service *hotels.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		hotelID, err := strconv.ParseInt(c.Param("hotelID"), 10, 64)
		if err != nil || hotelID < 1 {
			writeError(c, http.StatusBadRequest, "invalid_request", "hotelID must be a positive integer")
			return
		}

		input, err := parseAvailability(c, hotelID)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		rooms, err := service.AvailableRooms(c.Request.Context(), input)
		switch {
		case errors.Is(err, domain.ErrHotelNotFound):
			writeError(c, http.StatusNotFound, "hotel_not_found", "hotel not found")
			return
		case err != nil:
			writeError(c, http.StatusInternalServerError, "internal_error", "unable to find available rooms")
			return
		}

		response := make([]availableRoomResponse, len(rooms))
		for i, room := range rooms {
			response[i] = availableRoomResponse{ID: room.ID, Number: room.Number, Type: room.Type, Capacity: room.Capacity}
		}
		c.JSON(http.StatusOK, response)
	}
}

func parseAvailability(c *gin.Context, hotelID int64) (hotels.AvailabilityInput, error) {
	checkIn, err := parseDateQuery(c, "check_in")
	if err != nil {
		return hotels.AvailabilityInput{}, err
	}
	checkOut, err := parseDateQuery(c, "check_out")
	if err != nil {
		return hotels.AvailabilityInput{}, err
	}
	stay, err := domain.NewStay(checkIn, checkOut)
	if err != nil {
		return hotels.AvailabilityInput{}, errors.New("check_in must be before check_out and the stay may not exceed 30 nights")
	}

	guests, err := strconv.Atoi(strings.TrimSpace(c.Query("guests")))
	if err != nil || guests < 1 {
		return hotels.AvailabilityInput{}, errors.New("guests must be a positive integer")
	}
	return hotels.AvailabilityInput{HotelID: hotelID, Stay: stay, GuestCount: guests}, nil
}

func parseDateQuery(c *gin.Context, name string) (time.Time, error) {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return time.Time{}, errors.New(name + " query parameter is required")
	}
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, errors.New(name + " must use YYYY-MM-DD format")
	}
	return parsed, nil
}
