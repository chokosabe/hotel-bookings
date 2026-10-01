package evaluatordata_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/chokosabe/hotel-bookings/internal/database"
	"github.com/chokosabe/hotel-bookings/internal/evaluatordata"
	"github.com/chokosabe/hotel-bookings/internal/persistence"
)

func TestSeedIsIdempotentAndResetRemovesData(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access database pool: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	service := evaluatordata.NewService(db)
	if err := service.Seed(context.Background()); err != nil {
		t.Fatalf("first Seed() error = %v", err)
	}
	if err := service.Seed(context.Background()); err != nil {
		t.Fatalf("second Seed() error = %v", err)
	}

	var hotels, rooms int64
	if err := db.Model(&persistence.Hotel{}).Count(&hotels).Error; err != nil {
		t.Fatalf("count hotels: %v", err)
	}
	if err := db.Model(&persistence.Room{}).Count(&rooms).Error; err != nil {
		t.Fatalf("count rooms: %v", err)
	}
	if hotels != 1 || rooms != 6 {
		t.Fatalf("seeded %d hotels and %d rooms, want 1 and 6", hotels, rooms)
	}

	if err := service.Reset(context.Background()); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	if err := db.Model(&persistence.Hotel{}).Count(&hotels).Error; err != nil {
		t.Fatalf("count reset hotels: %v", err)
	}
	if hotels != 0 {
		t.Errorf("hotels after reset = %d, want 0", hotels)
	}
}
