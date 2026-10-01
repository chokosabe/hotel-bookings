package hotels_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/database"
	"github.com/chokosabe/hotel-bookings/internal/domain"
	"github.com/chokosabe/hotel-bookings/internal/evaluatordata"
	"github.com/chokosabe/hotel-bookings/internal/hotels"
)

func TestAvailableRoomsExcludesOccupiedNightsAndAllowsCheckoutReuse(t *testing.T) {
	db := seededDatabase(t)
	var roomID int64
	if err := db.QueryRow(`SELECT id FROM rooms WHERE number = '101'`).Scan(&roomID); err != nil {
		t.Fatalf("find room: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO bookings (reference, room_id, check_in, check_out, guest_count, lead_guest_name, lead_guest_email, created_at) VALUES ('HBK-EXISTING', ?, '2026-12-10', '2026-12-12', 1, 'Guest', 'guest@example.com', '2026-01-01T00:00:00Z')`, roomID); err != nil {
		t.Fatalf("insert booking: %v", err)
	}
	var bookingID int64
	if err := db.QueryRow(`SELECT id FROM bookings WHERE reference = 'HBK-EXISTING'`).Scan(&bookingID); err != nil {
		t.Fatalf("find booking: %v", err)
	}
	for _, date := range []string{"2026-12-10", "2026-12-11"} {
		if _, err := db.Exec(`INSERT INTO booking_nights (booking_id, room_id, stay_date) VALUES (?, ?, ?)`, bookingID, roomID, date); err != nil {
			t.Fatalf("insert booking night %s: %v", date, err)
		}
	}

	service := hotels.NewService(db)
	occupiedStay := mustStay(t, "2026-12-10", "2026-12-12")
	rooms, err := service.AvailableRooms(context.Background(), hotels.AvailabilityInput{HotelID: 1, Stay: occupiedStay, GuestCount: 1})
	if err != nil {
		t.Fatalf("AvailableRooms() error = %v", err)
	}
	if got, want := roomNumbers(rooms), []string{"102", "201", "202", "301", "302"}; !equalStrings(got, want) {
		t.Errorf("occupied-stay rooms = %v, want %v", got, want)
	}

	checkoutStay := mustStay(t, "2026-12-12", "2026-12-13")
	rooms, err = service.AvailableRooms(context.Background(), hotels.AvailabilityInput{HotelID: 1, Stay: checkoutStay, GuestCount: 1})
	if err != nil {
		t.Fatalf("checkout AvailableRooms() error = %v", err)
	}
	if got, want := roomNumbers(rooms), []string{"101", "102", "201", "202", "301", "302"}; !equalStrings(got, want) {
		t.Errorf("checkout-stay rooms = %v, want %v", got, want)
	}
}

func TestAvailableRoomsFiltersByCapacityAndReportsMissingHotel(t *testing.T) {
	db := seededDatabase(t)
	service := hotels.NewService(db)
	stay := mustStay(t, "2026-12-10", "2026-12-11")

	rooms, err := service.AvailableRooms(context.Background(), hotels.AvailabilityInput{HotelID: 1, Stay: stay, GuestCount: 3})
	if err != nil {
		t.Fatalf("AvailableRooms() error = %v", err)
	}
	if got, want := roomNumbers(rooms), []string{"301", "302"}; !equalStrings(got, want) {
		t.Errorf("capacity-filtered rooms = %v, want %v", got, want)
	}

	_, err = service.AvailableRooms(context.Background(), hotels.AvailabilityInput{HotelID: 999, Stay: stay, GuestCount: 1})
	if err != hotels.ErrHotelNotFound {
		t.Errorf("missing hotel error = %v, want ErrHotelNotFound", err)
	}
}

func seededDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := evaluatordata.NewService(db).Seed(context.Background()); err != nil {
		t.Fatalf("seed database: %v", err)
	}
	return db
}

func mustStay(t *testing.T, checkIn, checkOut string) domain.Stay {
	t.Helper()
	start, err := time.Parse("2006-01-02", checkIn)
	if err != nil {
		t.Fatalf("parse check-in: %v", err)
	}
	end, err := time.Parse("2006-01-02", checkOut)
	if err != nil {
		t.Fatalf("parse check-out: %v", err)
	}
	stay, err := domain.NewStay(start, end)
	if err != nil {
		t.Fatalf("NewStay() error = %v", err)
	}
	return stay
}

func roomNumbers(rooms []domain.Room) []string {
	numbers := make([]string, len(rooms))
	for i, room := range rooms {
		numbers[i] = room.Number
	}
	return numbers
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
