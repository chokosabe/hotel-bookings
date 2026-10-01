// Package domain contains business entities shared across application services.
package domain

// Hotel is a property that contains bookable rooms.
type Hotel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Room is a physical room belonging to a hotel.
type Room struct {
	ID       int64  `json:"id"`
	HotelID  int64  `json:"hotel_id"`
	Number   string `json:"number"`
	Type     string `json:"type"`
	Capacity int    `json:"capacity"`
}
