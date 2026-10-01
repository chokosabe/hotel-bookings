package database_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/chokosabe/hotel-bookings/internal/database"
	"github.com/chokosabe/hotel-bookings/internal/persistence"
)

func TestOpenAutoMigratesDomainTablesAndEnforcesForeignKeys(t *testing.T) {
	t.Parallel()

	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "hotel-bookings.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	closeDatabase(t, db)

	for _, model := range []any{
		&persistence.Hotel{},
		&persistence.Room{},
		&persistence.Booking{},
		&persistence.BookingNight{},
	} {
		if !db.Migrator().HasTable(model) {
			t.Errorf("migration did not create table for %T", model)
		}
	}

	err = db.Create(&persistence.Room{HotelID: 999, Number: "101", Type: "single", Capacity: 1}).Error
	if err == nil {
		t.Error("insert with missing hotel succeeded; foreign keys are not enabled")
	}
}

func closeDatabase(t *testing.T, db interface{ DB() (*sql.DB, error) }) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access database pool: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}
