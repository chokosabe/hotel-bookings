package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/domain"
)

func TestNewStayUsesHalfOpenCalendarDates(t *testing.T) {
	checkIn := time.Date(2026, time.December, 10, 0, 0, 0, 0, time.UTC)
	checkOut := time.Date(2026, time.December, 12, 0, 0, 0, 0, time.UTC)

	stay, err := domain.NewStay(checkIn, checkOut)
	if err != nil {
		t.Fatalf("NewStay() error = %v", err)
	}
	if stay.Nights() != 2 {
		t.Errorf("Nights() = %d, want 2", stay.Nights())
	}
}

func TestNewStayRejectsEmptyAndOverlongStays(t *testing.T) {
	checkIn := time.Date(2026, time.December, 10, 0, 0, 0, 0, time.UTC)
	for _, checkOut := range []time.Time{checkIn, checkIn.AddDate(0, 0, 31)} {
		if _, err := domain.NewStay(checkIn, checkOut); !errors.Is(err, domain.ErrInvalidStay) {
			t.Errorf("NewStay(%s) error = %v, want ErrInvalidStay", checkOut, err)
		}
	}
}
