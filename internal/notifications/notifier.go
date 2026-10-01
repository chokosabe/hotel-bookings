// Package notifications simulates confirmation delivery outside the booking transaction.
package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/domain"
)

// Notifier accepts confirmation work after a booking has committed.
type Notifier interface {
	NotifyBookingConfirmed(domain.Booking)
}

// AsyncNotifier logs a confirmation after a delay and owns its goroutine lifecycle.
type AsyncNotifier struct {
	ctx    context.Context
	cancel context.CancelFunc
	delay  time.Duration
	logger *slog.Logger
	wg     sync.WaitGroup
}

// NewAsyncNotifier creates a lifecycle-aware confirmation simulator.
func NewAsyncNotifier(logger *slog.Logger, delay time.Duration) *AsyncNotifier {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &AsyncNotifier{ctx: ctx, cancel: cancel, delay: delay, logger: logger}
}

// NotifyBookingConfirmed starts non-blocking simulated delivery.
func (n *AsyncNotifier) NotifyBookingConfirmed(booking domain.Booking) {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		timer := time.NewTimer(n.delay)
		defer timer.Stop()
		select {
		case <-n.ctx.Done():
			return
		case <-timer.C:
			n.logger.Info("booking confirmation email sent", "booking_reference", booking.Reference, "guest_email", booking.LeadGuestEmail)
		}
	}()
}

// Wait blocks for delivery already accepted by the notifier or until ctx expires.
func (n *AsyncNotifier) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		n.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for confirmation notifications: %w", ctx.Err())
	}
}

// Close cancels notification work that did not complete before shutdown.
func (n *AsyncNotifier) Close() {
	n.cancel()
}
