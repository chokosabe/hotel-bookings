package notifications_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/domain"
	"github.com/chokosabe/hotel-bookings/internal/notifications"
)

func TestAsyncNotifierLogsConfirmationAndWaitsForDelivery(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	notifier := notifications.NewAsyncNotifier(logger, time.Millisecond)
	t.Cleanup(notifier.Close)

	notifier.NotifyBookingConfirmed(domain.Booking{Reference: "HBK-ABCDEFGHIJKL", LeadGuestEmail: "guest@example.com"})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := notifier.Wait(ctx); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if !strings.Contains(output.String(), "HBK-ABCDEFGHIJKL") {
		t.Errorf("log = %s, want booking reference", output.String())
	}
}

func TestAsyncNotifierCloseCancelsPendingDelivery(t *testing.T) {
	var output bytes.Buffer
	notifier := notifications.NewAsyncNotifier(slog.New(slog.NewJSONHandler(&output, nil)), time.Hour)
	notifier.NotifyBookingConfirmed(domain.Booking{Reference: "HBK-CANCELLED", LeadGuestEmail: "guest@example.com"})
	notifier.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := notifier.Wait(ctx); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if output.Len() != 0 {
		t.Errorf("log = %s, want cancelled notification not to log", output.String())
	}
}
