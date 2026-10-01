// Package evaluatordata provides the deterministic evaluator dataset.
package evaluatordata

import (
	"context"
	"fmt"

	"github.com/chokosabe/hotel-bookings/internal/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SeedHotelID is the fixed identifier of the seeded hotel, so documented
// requests keep working after any number of reset/seed cycles.
const SeedHotelID int64 = 1

// Service resets and seeds data used to exercise the public API.
type Service struct {
	db *gorm.DB
}

// NewService constructs a test-data service backed by db.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Reset removes all booking nights, bookings, rooms, and hotels. Tables are
// cleared child-first because bookings deliberately do not cascade from rooms.
func (s *Service) Reset(ctx context.Context) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		all := tx.Session(&gorm.Session{AllowGlobalUpdate: true})
		for _, model := range []any{
			&persistence.BookingNight{},
			&persistence.Booking{},
			&persistence.Room{},
			&persistence.Hotel{},
		} {
			if err := all.Delete(model).Error; err != nil {
				return fmt.Errorf("reset %T: %w", model, err)
			}
		}
		return nil
	})
}

// Seed creates the deterministic hotel inventory with fixed identifiers. It is
// safe to invoke repeatedly.
func (s *Service) Seed(ctx context.Context) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var hotel persistence.Hotel
		if err := tx.Where(&persistence.Hotel{Name: "The Grand Hotel"}).
			Attrs(persistence.Hotel{ID: SeedHotelID}).
			FirstOrCreate(&hotel).Error; err != nil {
			return fmt.Errorf("insert hotel: %w", err)
		}

		rooms := []persistence.Room{
			{ID: 1, HotelID: hotel.ID, Number: "101", Type: "single", Capacity: 1},
			{ID: 2, HotelID: hotel.ID, Number: "102", Type: "single", Capacity: 1},
			{ID: 3, HotelID: hotel.ID, Number: "201", Type: "double", Capacity: 2},
			{ID: 4, HotelID: hotel.ID, Number: "202", Type: "double", Capacity: 2},
			{ID: 5, HotelID: hotel.ID, Number: "301", Type: "deluxe", Capacity: 4},
			{ID: 6, HotelID: hotel.ID, Number: "302", Type: "deluxe", Capacity: 4},
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rooms).Error; err != nil {
			return fmt.Errorf("insert rooms: %w", err)
		}
		return nil
	})
}
