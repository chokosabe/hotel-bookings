// Package evaluatordata provides the deterministic evaluator dataset.
package evaluatordata

import (
	"context"
	"database/sql"
	"fmt"
)

// Service resets and seeds data used to exercise the public API.
type Service struct {
	db *sql.DB
}

// NewService constructs a test-data service backed by db.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Reset removes all hotels and, through foreign keys, their rooms and bookings.
func (s *Service) Reset(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM hotels`); err != nil {
		return fmt.Errorf("reset hotel data: %w", err)
	}
	return nil
}

// Seed creates the deterministic hotel inventory. It is safe to invoke repeatedly.
func (s *Service) Seed(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO hotels (name) VALUES ('The Grand Hotel')`); err != nil {
		return fmt.Errorf("insert hotel: %w", err)
	}

	var hotelID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM hotels WHERE name = 'The Grand Hotel'`).Scan(&hotelID); err != nil {
		return fmt.Errorf("find seeded hotel: %w", err)
	}

	rooms := []struct {
		number   string
		roomType string
		capacity int
	}{
		{"101", "single", 1}, {"102", "single", 1},
		{"201", "double", 2}, {"202", "double", 2},
		{"301", "deluxe", 4}, {"302", "deluxe", 4},
	}
	for _, room := range rooms {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO rooms (hotel_id, number, type, capacity)
			VALUES (?, ?, ?, ?)
		`, hotelID, room.number, room.roomType, room.capacity); err != nil {
			return fmt.Errorf("insert room %s: %w", room.number, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}
	return nil
}
