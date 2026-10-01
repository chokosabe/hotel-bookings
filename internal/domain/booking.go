package domain

import "time"

// Booking is a confirmed stay in one room for its complete duration.
type Booking struct {
	ID             int64
	Reference      string
	HotelID        int64
	Room           Room
	Stay           Stay
	GuestCount     int
	LeadGuestName  string
	LeadGuestEmail string
	CreatedAt      time.Time
}
