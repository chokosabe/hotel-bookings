// Package evaluatordata provides the deterministic evaluator dataset.
package evaluatordata

import (
	"context"
	"fmt"

	"github.com/chokosabe/hotel-bookings/internal/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service resets and seeds data used to exercise the public API.
type Service struct {
	db *gorm.DB
}

// NewService constructs a test-data service backed by db.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Reset removes all hotels and, through foreign keys, their rooms and bookings.
func (s *Service) Reset(ctx context.Context) error {
	if err := s.db.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&persistence.Hotel{}).Error; err != nil {
		return fmt.Errorf("reset hotel data: %w", err)
	}
	return nil
}

// Seed creates the deterministic hotel inventory. It is safe to invoke repeatedly.
func (s *Service) Seed(ctx context.Context) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		hotel := persistence.Hotel{Name: "The Grand Hotel"}
		if err := tx.Where(&persistence.Hotel{Name: hotel.Name}).FirstOrCreate(&hotel).Error; err != nil {
			return fmt.Errorf("insert hotel: %w", err)
		}

		rooms := []persistence.Room{
			{HotelID: hotel.ID, Number: "101", Type: "single", Capacity: 1},
			{HotelID: hotel.ID, Number: "102", Type: "single", Capacity: 1},
			{HotelID: hotel.ID, Number: "201", Type: "double", Capacity: 2},
			{HotelID: hotel.ID, Number: "202", Type: "double", Capacity: 2},
			{HotelID: hotel.ID, Number: "301", Type: "deluxe", Capacity: 4},
			{HotelID: hotel.ID, Number: "302", Type: "deluxe", Capacity: 4},
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rooms).Error; err != nil {
			return fmt.Errorf("insert rooms: %w", err)
		}
		return nil
	})
}
