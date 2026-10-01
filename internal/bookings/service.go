// Package bookings implements atomic booking creation and lookup use cases.
package bookings

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/domain"
	"github.com/chokosabe/hotel-bookings/internal/notifications"
)

var (
	// ErrInvalidBooking means a create request violates a booking input rule.
	ErrInvalidBooking = errors.New("invalid booking")
	// ErrNoSuitableRoom means no room can accommodate the party for every requested night.
	ErrNoSuitableRoom = errors.New("no suitable room available")
	// ErrBookingNotFound means no booking has the requested public reference.
	ErrBookingNotFound = errors.New("booking not found")
)

const dateLayout = "2006-01-02"

// CreateInput contains all information needed to confirm a stay.
type CreateInput struct {
	HotelID        int64
	Stay           domain.Stay
	GuestCount     int
	LeadGuestName  string
	LeadGuestEmail string
}

// Service owns booking creation. It invokes notifier only after a committed booking.
type Service struct {
	db       *sql.DB
	notifier notifications.Notifier
	now      func() time.Time
}

// NewService constructs a booking service backed by db.
func NewService(db *sql.DB, notifier notifications.Notifier) *Service {
	return &Service{db: db, notifier: notifier, now: time.Now}
}

// Create atomically assigns a suitable room and reserves every night in input.Stay.
func (s *Service) Create(ctx context.Context, input CreateInput) (domain.Booking, error) {
	input, err := validate(input)
	if err != nil {
		return domain.Booking{}, err
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return domain.Booking{}, fmt.Errorf("begin booking transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var hotelExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hotels WHERE id = ?)`, input.HotelID).Scan(&hotelExists); err != nil {
		return domain.Booking{}, fmt.Errorf("find hotel: %w", err)
	}
	if !hotelExists {
		return domain.Booking{}, domain.ErrHotelNotFound
	}

	room, err := selectAvailableRoom(ctx, tx, input)
	if err != nil {
		return domain.Booking{}, err
	}

	createdAt := s.now().UTC()
	booking, err := insertBooking(ctx, tx, input, room, createdAt)
	if err != nil {
		return domain.Booking{}, err
	}
	if err := reserveNights(ctx, tx, booking.ID, room.ID, input.Stay); err != nil {
		return domain.Booking{}, err
	}
	if err := tx.Commit(); err != nil {
		if isUniqueConstraint(err) {
			return domain.Booking{}, ErrNoSuitableRoom
		}
		return domain.Booking{}, fmt.Errorf("commit booking transaction: %w", err)
	}

	if s.notifier != nil {
		s.notifier.NotifyBookingConfirmed(booking)
	}
	return booking, nil
}

func validate(input CreateInput) (CreateInput, error) {
	stay, err := domain.NewStay(input.Stay.CheckIn, input.Stay.CheckOut)
	if err != nil {
		return CreateInput{}, fmt.Errorf("%w: check_in must be before check_out and the stay may not exceed 30 nights", ErrInvalidBooking)
	}
	if input.HotelID < 1 {
		return CreateInput{}, fmt.Errorf("%w: hotel_id must be a positive integer", ErrInvalidBooking)
	}
	if input.GuestCount < 1 {
		return CreateInput{}, fmt.Errorf("%w: guest_count must be a positive integer", ErrInvalidBooking)
	}
	input.LeadGuestName = strings.TrimSpace(input.LeadGuestName)
	if input.LeadGuestName == "" {
		return CreateInput{}, fmt.Errorf("%w: lead_guest_name is required", ErrInvalidBooking)
	}
	input.LeadGuestEmail = strings.TrimSpace(input.LeadGuestEmail)
	address, err := mail.ParseAddress(input.LeadGuestEmail)
	if err != nil || address.Address != input.LeadGuestEmail {
		return CreateInput{}, fmt.Errorf("%w: lead_guest_email must be a valid email address", ErrInvalidBooking)
	}
	input.Stay = stay
	return input, nil
}

func selectAvailableRoom(ctx context.Context, tx *sql.Tx, input CreateInput) (domain.Room, error) {
	var room domain.Room
	err := tx.QueryRowContext(ctx, `
		SELECT r.id, r.hotel_id, r.number, r.type, r.capacity
		FROM rooms r
		WHERE r.hotel_id = ?
		  AND r.capacity >= ?
		  AND NOT EXISTS (
		      SELECT 1 FROM booking_nights bn
		      WHERE bn.room_id = r.id
		        AND bn.stay_date >= ?
		        AND bn.stay_date < ?
		  )
		ORDER BY r.capacity ASC, CAST(r.number AS INTEGER) ASC, r.number ASC
		LIMIT 1
	`, input.HotelID, input.GuestCount, input.Stay.CheckIn.Format(dateLayout), input.Stay.CheckOut.Format(dateLayout)).Scan(
		&room.ID, &room.HotelID, &room.Number, &room.Type, &room.Capacity,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Room{}, ErrNoSuitableRoom
	}
	if err != nil {
		return domain.Room{}, fmt.Errorf("select available room: %w", err)
	}
	return room, nil
}

func insertBooking(ctx context.Context, tx *sql.Tx, input CreateInput, room domain.Room, createdAt time.Time) (domain.Booking, error) {
	for range 3 {
		reference, err := newReference()
		if err != nil {
			return domain.Booking{}, fmt.Errorf("generate booking reference: %w", err)
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO bookings (reference, room_id, check_in, check_out, guest_count, lead_guest_name, lead_guest_email, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, reference, room.ID, input.Stay.CheckIn.Format(dateLayout), input.Stay.CheckOut.Format(dateLayout), input.GuestCount, input.LeadGuestName, input.LeadGuestEmail, createdAt.Format(time.RFC3339Nano))
		if err != nil {
			if isReferenceConflict(err) {
				continue
			}
			return domain.Booking{}, fmt.Errorf("insert booking: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return domain.Booking{}, fmt.Errorf("read booking ID: %w", err)
		}
		return domain.Booking{ID: id, Reference: reference, HotelID: input.HotelID, Room: room, Stay: input.Stay, GuestCount: input.GuestCount, LeadGuestName: input.LeadGuestName, LeadGuestEmail: input.LeadGuestEmail, CreatedAt: createdAt}, nil
	}
	return domain.Booking{}, errors.New("generate unique booking reference after three collisions")
}

func reserveNights(ctx context.Context, tx *sql.Tx, bookingID, roomID int64, stay domain.Stay) error {
	for night := stay.CheckIn; night.Before(stay.CheckOut); night = night.AddDate(0, 0, 1) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO booking_nights (booking_id, room_id, stay_date) VALUES (?, ?, ?)`, bookingID, roomID, night.Format(dateLayout)); err != nil {
			if isUniqueConstraint(err) {
				return ErrNoSuitableRoom
			}
			return fmt.Errorf("reserve night %s: %w", night.Format(dateLayout), err)
		}
	}
	return nil
}

// FindByReference returns a confirmed booking and its assigned room.
func (s *Service) FindByReference(ctx context.Context, reference string) (domain.Booking, error) {
	var booking domain.Booking
	var checkIn, checkOut, createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT b.id, b.reference, b.room_id, r.hotel_id, r.number, r.type, r.capacity,
		       b.check_in, b.check_out, b.guest_count, b.lead_guest_name, b.lead_guest_email, b.created_at
		FROM bookings b
		JOIN rooms r ON r.id = b.room_id
		WHERE b.reference = ?
	`, strings.TrimSpace(reference)).Scan(
		&booking.ID, &booking.Reference, &booking.Room.ID, &booking.HotelID, &booking.Room.Number, &booking.Room.Type, &booking.Room.Capacity,
		&checkIn, &checkOut, &booking.GuestCount, &booking.LeadGuestName, &booking.LeadGuestEmail, &createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Booking{}, ErrBookingNotFound
	}
	if err != nil {
		return domain.Booking{}, fmt.Errorf("find booking: %w", err)
	}
	booking.Room.HotelID = booking.HotelID
	checkInDate, err := time.Parse(dateLayout, checkIn)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("parse stored check-in: %w", err)
	}
	checkOutDate, err := time.Parse(dateLayout, checkOut)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("parse stored check-out: %w", err)
	}
	booking.Stay, err = domain.NewStay(checkInDate, checkOutDate)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("validate stored stay: %w", err)
	}
	booking.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("parse stored creation time: %w", err)
	}
	return booking, nil
}

func newReference() (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i := range bytes {
		bytes[i] = alphabet[int(bytes[i])%len(alphabet)]
	}
	return "HBK-" + string(bytes), nil
}

func isReferenceConflict(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint failed: bookings.reference")
}

func isUniqueConstraint(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}
