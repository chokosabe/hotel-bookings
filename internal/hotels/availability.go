package hotels

import (
	"context"
	"errors"
	"fmt"

	"github.com/chokosabe/hotel-bookings/internal/domain"
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

	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hotels WHERE id = ?)`, input.HotelID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("find hotel: %w", err)
	}
	if !exists {
		return nil, domain.ErrHotelNotFound
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.hotel_id, r.number, r.type, r.capacity
		FROM rooms r
		WHERE r.hotel_id = ?
		  AND r.capacity >= ?
		  AND NOT EXISTS (
		      SELECT 1
		      FROM booking_nights bn
		      WHERE bn.room_id = r.id
		        AND bn.stay_date >= ?
		        AND bn.stay_date < ?
		  )
		ORDER BY r.capacity ASC, CAST(r.number AS INTEGER) ASC, r.number ASC
	`, input.HotelID, input.GuestCount, input.Stay.CheckIn.Format(timeLayout), input.Stay.CheckOut.Format(timeLayout))
	if err != nil {
		return nil, fmt.Errorf("find available rooms: %w", err)
	}
	defer rows.Close()

	rooms := make([]domain.Room, 0)
	for rows.Next() {
		var room domain.Room
		if err := rows.Scan(&room.ID, &room.HotelID, &room.Number, &room.Type, &room.Capacity); err != nil {
			return nil, fmt.Errorf("scan available room: %w", err)
		}
		rooms = append(rooms, room)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate available rooms: %w", err)
	}
	return rooms, nil
}

const timeLayout = "2006-01-02"
