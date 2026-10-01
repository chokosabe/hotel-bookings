// Package bookings implements atomic booking creation and lookup use cases.
package bookings

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/domain"
	"github.com/chokosabe/hotel-bookings/internal/notifications"
	"github.com/chokosabe/hotel-bookings/internal/persistence"
	"gorm.io/gorm"
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
	db       *gorm.DB
	notifier notifications.Notifier
	now      func() time.Time
}

// NewService constructs a booking service backed by db.
func NewService(db *gorm.DB, notifier notifications.Notifier) *Service {
	return &Service{db: db, notifier: notifier, now: time.Now}
}

// Create atomically assigns a suitable room and reserves every night in input.Stay.
func (s *Service) Create(ctx context.Context, input CreateInput) (domain.Booking, error) {
	input, err := validate(input)
	if err != nil {
		return domain.Booking{}, err
	}

	var booking domain.Booking
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var hotel persistence.Hotel
		if err := tx.First(&hotel, input.HotelID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrHotelNotFound
			}
			return fmt.Errorf("find hotel: %w", err)
		}

		room, err := selectAvailableRoom(tx, input)
		if err != nil {
			return err
		}

		booking, err = insertBooking(tx, input, room, s.now().UTC())
		if err != nil {
			return err
		}
		if err := reserveNights(tx, booking.ID, room.ID, input.Stay); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return domain.Booking{}, ErrNoSuitableRoom
		}
		return domain.Booking{}, err
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

func selectAvailableRoom(tx *gorm.DB, input CreateInput) (domain.Room, error) {
	occupied := tx.Model(&persistence.BookingNight{}).
		Select("1").
		Where("booking_nights.room_id = rooms.id").
		Where("stay_date >= ? AND stay_date < ?", input.Stay.CheckIn.Format(dateLayout), input.Stay.CheckOut.Format(dateLayout))
	var room persistence.Room
	result := tx.Where("hotel_id = ? AND capacity >= ?", input.HotelID, input.GuestCount).
		Where("NOT EXISTS (?)", occupied).
		Order("capacity ASC").
		Order("CAST(number AS INTEGER) ASC").
		Order("number ASC").
		Limit(1).
		Find(&room)
	if result.Error != nil {
		return domain.Room{}, fmt.Errorf("select available room: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.Room{}, ErrNoSuitableRoom
	}
	return persistence.RoomToDomain(room), nil
}

func insertBooking(tx *gorm.DB, input CreateInput, room domain.Room, createdAt time.Time) (domain.Booking, error) {
	for range 3 {
		reference, err := newReference()
		if err != nil {
			return domain.Booking{}, fmt.Errorf("generate booking reference: %w", err)
		}
		record := persistence.Booking{
			Reference:      reference,
			RoomID:         room.ID,
			CheckIn:        input.Stay.CheckIn.Format(dateLayout),
			CheckOut:       input.Stay.CheckOut.Format(dateLayout),
			GuestCount:     input.GuestCount,
			LeadGuestName:  input.LeadGuestName,
			LeadGuestEmail: input.LeadGuestEmail,
			CreatedAt:      createdAt.Format(time.RFC3339Nano),
		}
		if err := tx.Create(&record).Error; err != nil {
			if isReferenceConflict(err) {
				continue
			}
			return domain.Booking{}, fmt.Errorf("insert booking: %w", err)
		}
		return domain.Booking{ID: record.ID, Reference: reference, HotelID: input.HotelID, Room: room, Stay: input.Stay, GuestCount: input.GuestCount, LeadGuestName: input.LeadGuestName, LeadGuestEmail: input.LeadGuestEmail, CreatedAt: createdAt}, nil
	}
	return domain.Booking{}, errors.New("generate unique booking reference after three collisions")
}

func reserveNights(tx *gorm.DB, bookingID, roomID int64, stay domain.Stay) error {
	for night := stay.CheckIn; night.Before(stay.CheckOut); night = night.AddDate(0, 0, 1) {
		record := persistence.BookingNight{BookingID: bookingID, RoomID: roomID, StayDate: night.Format(dateLayout)}
		if err := tx.Create(&record).Error; err != nil {
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
	var record persistence.Booking
	if err := s.db.WithContext(ctx).Preload("Room").Where("reference = ?", strings.TrimSpace(reference)).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Booking{}, ErrBookingNotFound
		}
		return domain.Booking{}, fmt.Errorf("find booking: %w", err)
	}
	booking, err := persistence.BookingToDomain(record)
	if err != nil {
		return domain.Booking{}, err
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
	return errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique constraint failed: bookings.reference")
}

func isUniqueConstraint(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}
