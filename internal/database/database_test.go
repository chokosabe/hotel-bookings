package database_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/chokosabe/hotel-bookings/internal/database"
)

func TestOpenAppliesMigrationsAndForeignKeys(t *testing.T) {
	t.Parallel()

	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "hotel-bookings.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var tableCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('hotels', 'rooms', 'bookings', 'booking_nights')`).Scan(&tableCount); err != nil {
		t.Fatalf("query schema: %v", err)
	}
	if tableCount != 4 {
		t.Errorf("domain table count = %d, want 4", tableCount)
	}

	if _, err := db.Exec(`INSERT INTO rooms (hotel_id, number, type, capacity) VALUES (999, '101', 'single', 1)`); err == nil {
		t.Error("insert with missing hotel succeeded; foreign keys are not enabled")
	}
}
