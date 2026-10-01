package bookings_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/bookings"
	"github.com/chokosabe/hotel-bookings/internal/database"
	"github.com/chokosabe/hotel-bookings/internal/domain"
	"github.com/chokosabe/hotel-bookings/internal/evaluatordata"
	"github.com/chokosabe/hotel-bookings/internal/persistence"
	"gorm.io/gorm"
)

func TestCreateAssignsLowestAdequateRoomReservesEveryNightAndNotifies(t *testing.T) {
	db := bookingDatabase(t)
	notifier := &recordingNotifier{}
	service := bookings.NewService(db, notifier)

	booking, err := service.Create(context.Background(), validInput(t))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if booking.Room.Number != "101" {
		t.Errorf("assigned room = %s, want 101", booking.Room.Number)
	}
	if len(booking.Reference) != len("HBK-")+12 || booking.Reference[:4] != "HBK-" {
		t.Errorf("reference = %q, want HBK- plus 12 characters", booking.Reference)
	}
	var nights int64
	if err := db.Model(&persistence.BookingNight{}).Where("booking_id = ?", booking.ID).Count(&nights).Error; err != nil {
		t.Fatalf("count booking nights: %v", err)
	}
	if nights != 2 {
		t.Errorf("booking nights = %d, want 2", nights)
	}
	if notifier.count() != 1 {
		t.Errorf("notifier calls = %d, want 1", notifier.count())
	}
}

func TestFindByReferenceReturnsTheAssignedRoom(t *testing.T) {
	db := bookingDatabase(t)
	service := bookings.NewService(db, nil)
	created, err := service.Create(context.Background(), validInput(t))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	found, err := service.FindByReference(context.Background(), created.Reference)
	if err != nil {
		t.Fatalf("FindByReference() error = %v", err)
	}
	if found.Reference != created.Reference || found.Room.Number != "101" || found.LeadGuestEmail != "ada@example.com" {
		t.Errorf("found booking = %#v, want created booking details", found)
	}
	_, err = service.FindByReference(context.Background(), "HBK-NOTFOUND")
	if !errors.Is(err, bookings.ErrBookingNotFound) {
		t.Errorf("missing booking error = %v, want ErrBookingNotFound", err)
	}
}

func TestCreateRejectsInvalidInputWithoutNotifying(t *testing.T) {
	db := bookingDatabase(t)
	notifier := &recordingNotifier{}
	service := bookings.NewService(db, notifier)
	input := validInput(t)
	input.LeadGuestEmail = "not-an-email"

	_, err := service.Create(context.Background(), input)
	if !errors.Is(err, bookings.ErrInvalidBooking) {
		t.Fatalf("Create() error = %v, want ErrInvalidBooking", err)
	}
	if notifier.count() != 0 {
		t.Errorf("notifier calls = %d, want 0", notifier.count())
	}
}

func TestCompetingBookingsForFinalRoomYieldOneSuccess(t *testing.T) {
	db := bookingDatabase(t)
	if err := db.Where("number <> ?", "101").Delete(&persistence.Room{}).Error; err != nil {
		t.Fatalf("reduce inventory: %v", err)
	}
	service := bookings.NewService(db, nil)
	input := validInput(t)

	results := make(chan error, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := service.Create(context.Background(), input)
			results <- err
		}()
	}
	group.Wait()
	close(results)

	var successes, conflicts int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, bookings.ErrNoSuitableRoom):
			conflicts++
		default:
			t.Errorf("unexpected competing booking error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Errorf("successes = %d, conflicts = %d, want 1 and 1", successes, conflicts)
	}
}

func bookingDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access database pool: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := evaluatordata.NewService(db).Seed(context.Background()); err != nil {
		t.Fatalf("seed database: %v", err)
	}
	return db
}

func validInput(t *testing.T) bookings.CreateInput {
	t.Helper()
	checkIn, err := time.Parse("2006-01-02", "2026-12-10")
	if err != nil {
		t.Fatalf("parse check-in: %v", err)
	}
	checkOut, err := time.Parse("2006-01-02", "2026-12-12")
	if err != nil {
		t.Fatalf("parse check-out: %v", err)
	}
	stay, err := domain.NewStay(checkIn, checkOut)
	if err != nil {
		t.Fatalf("create stay: %v", err)
	}
	return bookings.CreateInput{HotelID: 1, Stay: stay, GuestCount: 1, LeadGuestName: "Ada Lovelace", LeadGuestEmail: "ada@example.com"}
}

type recordingNotifier struct {
	mu       sync.Mutex
	bookings []domain.Booking
}

func (n *recordingNotifier) NotifyBookingConfirmed(booking domain.Booking) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.bookings = append(n.bookings, booking)
}

func (n *recordingNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.bookings)
}
