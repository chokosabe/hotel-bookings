package evaluatordata_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/bookings"
	"github.com/chokosabe/hotel-bookings/internal/database"
	"github.com/chokosabe/hotel-bookings/internal/domain"
	"github.com/chokosabe/hotel-bookings/internal/evaluatordata"
	"github.com/chokosabe/hotel-bookings/internal/persistence"
	"gorm.io/gorm"
)

func TestSeedIsIdempotentAndResetRemovesData(t *testing.T) {
	db := openDatabase(t)
	service := evaluatordata.NewService(db)
	if err := service.Seed(context.Background()); err != nil {
		t.Fatalf("first Seed() error = %v", err)
	}
	if err := service.Seed(context.Background()); err != nil {
		t.Fatalf("second Seed() error = %v", err)
	}

	if hotels, rooms := count(t, db, &persistence.Hotel{}), count(t, db, &persistence.Room{}); hotels != 1 || rooms != 6 {
		t.Fatalf("seeded %d hotels and %d rooms, want 1 and 6", hotels, rooms)
	}

	if err := service.Reset(context.Background()); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	if hotels := count(t, db, &persistence.Hotel{}); hotels != 0 {
		t.Errorf("hotels after reset = %d, want 0", hotels)
	}
}

func TestResetRemovesBookingsAndReseedKeepsIdentifiers(t *testing.T) {
	db := openDatabase(t)
	service := evaluatordata.NewService(db)
	ctx := context.Background()

	for cycle := 1; cycle <= 2; cycle++ {
		if err := service.Seed(ctx); err != nil {
			t.Fatalf("cycle %d: Seed() error = %v", cycle, err)
		}
		assertSeedIdentifiers(t, db, cycle)

		stay, err := domain.NewStay(time.Date(2026, time.December, 10, 0, 0, 0, 0, time.UTC), time.Date(2026, time.December, 12, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("NewStay() error = %v", err)
		}
		input := bookings.CreateInput{HotelID: evaluatordata.SeedHotelID, Stay: stay, GuestCount: 1, LeadGuestName: "Ada Lovelace", LeadGuestEmail: "ada@example.com"}
		if _, err := bookings.NewService(db, nil).Create(ctx, input); err != nil {
			t.Fatalf("cycle %d: Create() error = %v", cycle, err)
		}

		if err := service.Reset(ctx); err != nil {
			t.Fatalf("cycle %d: Reset() with an existing booking error = %v", cycle, err)
		}
		for _, model := range []any{&persistence.BookingNight{}, &persistence.Booking{}, &persistence.Room{}, &persistence.Hotel{}} {
			if n := count(t, db, model); n != 0 {
				t.Errorf("cycle %d: %T rows after reset = %d, want 0", cycle, model, n)
			}
		}
	}
}

func assertSeedIdentifiers(t *testing.T, db *gorm.DB, cycle int) {
	t.Helper()
	var hotel persistence.Hotel
	if err := db.First(&hotel).Error; err != nil {
		t.Fatalf("cycle %d: find hotel: %v", cycle, err)
	}
	if hotel.ID != evaluatordata.SeedHotelID {
		t.Errorf("cycle %d: hotel ID = %d, want %d", cycle, hotel.ID, evaluatordata.SeedHotelID)
	}
	var rooms []persistence.Room
	if err := db.Order("id").Find(&rooms).Error; err != nil {
		t.Fatalf("cycle %d: find rooms: %v", cycle, err)
	}
	for i, room := range rooms {
		if room.ID != int64(i+1) {
			t.Errorf("cycle %d: room %s ID = %d, want %d", cycle, room.Number, room.ID, i+1)
		}
	}
}

func openDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access database pool: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func count(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	if err := db.Model(model).Count(&n).Error; err != nil {
		t.Fatalf("count %T: %v", model, err)
	}
	return n
}
