// Package persistence contains GORM models and domain conversion helpers.
package persistence

import (
	"fmt"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/domain"
)

const DateLayout = "2006-01-02"

// Hotel is the database representation of a hotel.
type Hotel struct {
	ID    int64  `gorm:"primaryKey"`
	Name  string `gorm:"not null;unique"`
	Rooms []Room `gorm:"foreignKey:HotelID;constraint:OnDelete:CASCADE"`
}

func (Hotel) TableName() string { return "hotels" }

// Room is the database representation of a physical hotel room.
type Room struct {
	ID       int64  `gorm:"primaryKey"`
	HotelID  int64  `gorm:"not null;uniqueIndex:idx_rooms_hotel_number"`
	Number   string `gorm:"not null;uniqueIndex:idx_rooms_hotel_number"`
	Type     string `gorm:"not null;check:chk_rooms_type,type IN ('single','double','deluxe')"`
	Capacity int    `gorm:"not null;check:chk_rooms_capacity,capacity > 0"`
}

func (Room) TableName() string { return "rooms" }

// Booking is the database representation of a confirmed booking.
type Booking struct {
	ID             int64          `gorm:"primaryKey"`
	Reference      string         `gorm:"not null;unique"`
	RoomID         int64          `gorm:"not null"`
	Room           Room           `gorm:"foreignKey:RoomID"`
	CheckIn        string         `gorm:"not null"`
	CheckOut       string         `gorm:"not null"`
	GuestCount     int            `gorm:"not null;check:chk_bookings_guest_count,guest_count > 0"`
	LeadGuestName  string         `gorm:"not null"`
	LeadGuestEmail string         `gorm:"not null"`
	CreatedAt      string         `gorm:"not null"`
	Nights         []BookingNight `gorm:"foreignKey:BookingID;constraint:OnDelete:CASCADE"`
}

func (Booking) TableName() string { return "bookings" }

// BookingNight materializes each night occupied by a booking.
type BookingNight struct {
	BookingID int64   `gorm:"primaryKey"`
	Booking   Booking `gorm:"foreignKey:BookingID;constraint:OnDelete:CASCADE"`
	RoomID    int64   `gorm:"not null;uniqueIndex:idx_booking_nights_room_date"`
	Room      Room    `gorm:"foreignKey:RoomID;constraint:OnDelete:CASCADE"`
	StayDate  string  `gorm:"primaryKey;uniqueIndex:idx_booking_nights_room_date"`
}

func (BookingNight) TableName() string { return "booking_nights" }

func HotelToDomain(hotel Hotel) domain.Hotel {
	return domain.Hotel{ID: hotel.ID, Name: hotel.Name}
}

func RoomToDomain(room Room) domain.Room {
	return domain.Room{ID: room.ID, HotelID: room.HotelID, Number: room.Number, Type: room.Type, Capacity: room.Capacity}
}

// BookingToDomain converts a fully populated booking record to the domain model.
func BookingToDomain(booking Booking) (domain.Booking, error) {
	checkIn, err := time.Parse(DateLayout, booking.CheckIn)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("parse stored check-in: %w", err)
	}
	checkOut, err := time.Parse(DateLayout, booking.CheckOut)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("parse stored check-out: %w", err)
	}
	stay, err := domain.NewStay(checkIn, checkOut)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("validate stored stay: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, booking.CreatedAt)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("parse stored creation time: %w", err)
	}
	return domain.Booking{ID: booking.ID, Reference: booking.Reference, HotelID: booking.Room.HotelID, Room: RoomToDomain(booking.Room), Stay: stay, GuestCount: booking.GuestCount, LeadGuestName: booking.LeadGuestName, LeadGuestEmail: booking.LeadGuestEmail, CreatedAt: createdAt}, nil
}
