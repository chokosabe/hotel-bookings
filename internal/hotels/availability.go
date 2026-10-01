package hotels

import (
	"context"
	"errors"
	"fmt"

	"github.com/chokosabe/hotel-bookings/internal/domain"
	"github.com/chokosabe/hotel-bookings/internal/persistence"
	"gorm.io/gorm"
)

var (
	// ErrInvalidGuestCount means a party size cannot be used for room selection.
	ErrInvalidGuestCount = errors.New("guest count must be at least one")
)

// AvailabilityInput captures the criteria for a room search.
type AvailabilityInput struct {
	HotelID    int64
	Stay       domain.Stay
	GuestCount int
}

// AvailableRooms returns rooms that can contain the party for every night of the stay.
func (s *Service) AvailableRooms(ctx context.Context, input AvailabilityInput) ([]domain.Room, error) {
	stay, err := domain.NewStay(input.Stay.CheckIn, input.Stay.CheckOut)
	if err != nil {
		return nil, err
	}
	input.Stay = stay
	if input.GuestCount < 1 {
		return nil, ErrInvalidGuestCount
	}

	var hotel persistence.Hotel
	if err := s.db.WithContext(ctx).First(&hotel, input.HotelID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrHotelNotFound
		}
		return nil, fmt.Errorf("find hotel: %w", err)
	}

	occupied := s.db.WithContext(ctx).Model(&persistence.BookingNight{}).
		Select("1").
		Where("booking_nights.room_id = rooms.id").
		Where("stay_date >= ? AND stay_date < ?", input.Stay.CheckIn.Format(timeLayout), input.Stay.CheckOut.Format(timeLayout))
	var rows []persistence.Room
	if err := s.db.WithContext(ctx).
		Where("hotel_id = ? AND capacity >= ?", input.HotelID, input.GuestCount).
		Where("NOT EXISTS (?)", occupied).
		Order("capacity ASC").
		Order("CAST(number AS INTEGER) ASC").
		Order("number ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find available rooms: %w", err)
	}

	rooms := make([]domain.Room, len(rows))
	for i, room := range rows {
		rooms[i] = persistence.RoomToDomain(room)
	}
	return rooms, nil
}

const timeLayout = "2006-01-02"
