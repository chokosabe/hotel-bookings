package persistence_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/chokosabe/hotel-bookings/internal/database"
	"github.com/chokosabe/hotel-bookings/internal/persistence"
)

func TestBookingNightsEnforceUniqueRoomDate(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access database pool: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	hotel := persistence.Hotel{Name: "The Grand Hotel"}
	if err := db.Create(&hotel).Error; err != nil {
		t.Fatalf("create hotel: %v", err)
	}
	room := persistence.Room{HotelID: hotel.ID, Number: "101", Type: "single", Capacity: 1}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}
	first := persistence.Booking{Reference: "HBK-FIRST", RoomID: room.ID, CheckIn: "2026-12-10", CheckOut: "2026-12-11", GuestCount: 1, LeadGuestName: "Ada", LeadGuestEmail: "ada@example.com", CreatedAt: "2026-01-01T00:00:00Z"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first booking: %v", err)
	}
	second := persistence.Booking{Reference: "HBK-SECOND", RoomID: room.ID, CheckIn: "2026-12-10", CheckOut: "2026-12-11", GuestCount: 1, LeadGuestName: "Grace", LeadGuestEmail: "grace@example.com", CreatedAt: "2026-01-01T00:00:00Z"}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second booking: %v", err)
	}
	if err := db.Create(&persistence.BookingNight{BookingID: first.ID, RoomID: room.ID, StayDate: "2026-12-10"}).Error; err != nil {
		t.Fatalf("create first booking night: %v", err)
	}
	if err := db.Create(&persistence.BookingNight{BookingID: second.ID, RoomID: room.ID, StayDate: "2026-12-10"}).Error; err == nil {
		t.Error("duplicate room night unexpectedly succeeded")
	}
}
