package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/bookings"
	"github.com/chokosabe/hotel-bookings/internal/domain"
	"github.com/gin-gonic/gin"
)

const maxBookingBodyBytes = 1 << 20

type createBookingRequest struct {
	HotelID        int64
	CheckIn        string
	CheckOut       string
	GuestCount     int
	LeadGuestName  string
	LeadGuestEmail string
}

type bookingResponse struct {
	Reference      string                `json:"reference"`
	HotelID        int64                 `json:"hotel_id"`
	Room           availableRoomResponse `json:"room"`
	CheckIn        string                `json:"check_in"`
	CheckOut       string                `json:"check_out"`
	GuestCount     int                   `json:"guest_count"`
	LeadGuestName  string                `json:"lead_guest_name"`
	LeadGuestEmail string                `json:"lead_guest_email"`
	CreatedAt      time.Time             `json:"created_at"`
}

func createBooking(service *bookings.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		request, err := decodeCreateBookingRequest(c)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		input, err := request.toInput()
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}

		booking, err := service.Create(c.Request.Context(), input)
		switch {
		case errors.Is(err, domain.ErrHotelNotFound):
			writeError(c, http.StatusNotFound, "hotel_not_found", "hotel not found")
			return
		case errors.Is(err, bookings.ErrNoSuitableRoom):
			writeError(c, http.StatusConflict, "room_unavailable", "no suitable room is available for the requested stay")
			return
		case errors.Is(err, bookings.ErrInvalidBooking):
			writeError(c, http.StatusBadRequest, "invalid_request", strings.TrimPrefix(err.Error(), "invalid booking: "))
			return
		case err != nil:
			writeError(c, http.StatusInternalServerError, "internal_error", "unable to create booking")
			return
		}
		c.JSON(http.StatusCreated, newBookingResponse(booking))
	}
}

func findBooking(service *bookings.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		booking, err := service.FindByReference(c.Request.Context(), c.Param("reference"))
		switch {
		case errors.Is(err, bookings.ErrBookingNotFound):
			writeError(c, http.StatusNotFound, "booking_not_found", "booking not found")
			return
		case err != nil:
			writeError(c, http.StatusInternalServerError, "internal_error", "unable to find booking")
			return
		}
		c.JSON(http.StatusOK, newBookingResponse(booking))
	}
}

func decodeCreateBookingRequest(c *gin.Context) (createBookingRequest, error) {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		return createBookingRequest{}, errors.New("Content-Type must be application/json")
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBookingBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	first, err := decoder.Token()
	if err != nil {
		return createBookingRequest{}, errors.New("request body must be a JSON object")
	}
	if delimiter, ok := first.(json.Delim); !ok || delimiter != '{' {
		return createBookingRequest{}, errors.New("request body must be a JSON object")
	}

	var request createBookingRequest
	seen := make(map[string]struct{})
	for decoder.More() {
		field, err := decoder.Token()
		if err != nil {
			return createBookingRequest{}, errors.New("request body contains invalid JSON")
		}
		name, ok := field.(string)
		if !ok {
			return createBookingRequest{}, errors.New("request body contains invalid JSON")
		}
		if _, duplicate := seen[name]; duplicate {
			return createBookingRequest{}, fmt.Errorf("request body contains duplicate field %q", name)
		}
		seen[name] = struct{}{}

		switch name {
		case "hotel_id":
			err = decoder.Decode(&request.HotelID)
		case "check_in":
			err = decoder.Decode(&request.CheckIn)
		case "check_out":
			err = decoder.Decode(&request.CheckOut)
		case "guest_count":
			err = decoder.Decode(&request.GuestCount)
		case "lead_guest_name":
			err = decoder.Decode(&request.LeadGuestName)
		case "lead_guest_email":
			err = decoder.Decode(&request.LeadGuestEmail)
		default:
			return createBookingRequest{}, fmt.Errorf("request body contains unknown field %q", name)
		}
		if err != nil {
			return createBookingRequest{}, fmt.Errorf("field %q has an invalid value", name)
		}
	}
	last, err := decoder.Token()
	if err != nil {
		return createBookingRequest{}, errors.New("request body contains invalid JSON")
	}
	if delimiter, ok := last.(json.Delim); !ok || delimiter != '}' {
		return createBookingRequest{}, errors.New("request body contains invalid JSON")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return createBookingRequest{}, errors.New("request body must contain exactly one JSON object")
	}
	return request, nil
}

func (r createBookingRequest) toInput() (bookings.CreateInput, error) {
	checkIn, err := time.Parse(dateLayout, r.CheckIn)
	if err != nil {
		return bookings.CreateInput{}, errors.New("check_in must use YYYY-MM-DD format")
	}
	checkOut, err := time.Parse(dateLayout, r.CheckOut)
	if err != nil {
		return bookings.CreateInput{}, errors.New("check_out must use YYYY-MM-DD format")
	}
	stay, err := domain.NewStay(checkIn, checkOut)
	if err != nil {
		return bookings.CreateInput{}, errors.New("check_in must be before check_out and the stay may not exceed 30 nights")
	}
	return bookings.CreateInput{HotelID: r.HotelID, Stay: stay, GuestCount: r.GuestCount, LeadGuestName: r.LeadGuestName, LeadGuestEmail: r.LeadGuestEmail}, nil
}

func newBookingResponse(booking domain.Booking) bookingResponse {
	return bookingResponse{
		Reference: booking.Reference,
		HotelID:   booking.HotelID,
		Room: availableRoomResponse{
			ID: booking.Room.ID, Number: booking.Room.Number, Type: booking.Room.Type, Capacity: booking.Room.Capacity,
		},
		CheckIn:        booking.Stay.CheckIn.Format(dateLayout),
		CheckOut:       booking.Stay.CheckOut.Format(dateLayout),
		GuestCount:     booking.GuestCount,
		LeadGuestName:  booking.LeadGuestName,
		LeadGuestEmail: booking.LeadGuestEmail,
		CreatedAt:      booking.CreatedAt,
	}
}
