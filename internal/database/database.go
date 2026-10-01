// Package database owns GORM SQLite connectivity and schema migration.
package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chokosabe/hotel-bookings/internal/persistence"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Open opens a SQLite database, configures its connection pool, and creates any
// missing tables, indexes, and constraints from the GORM persistence models.
func Open(ctx context.Context, path string) (*gorm.DB, error) {
	if err := ensureParentDirectory(path); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(sqliteDSN(path)), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access database pool: %w", err)
	}
	// SQLite applies PRAGMAs per physical connection. One connection keeps the
	// settings stable and matches this deliberately single-file deployment.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.WithContext(ctx).AutoMigrate(
		&persistence.Hotel{},
		&persistence.Room{},
		&persistence.Booking{},
		&persistence.BookingNight{},
	); err != nil {
		return nil, fmt.Errorf("auto-migrate database: %w", err)
	}
	return db, nil
}

func sqliteDSN(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
}

func ensureParentDirectory(path string) error {
	if path == ":memory:" || strings.HasPrefix(path, "file:") {
		return nil
	}
	return os.MkdirAll(filepath.Dir(path), 0o750)
}
