package domain

import (
	"errors"
	"time"
)

const MaxStayNights = 30

var ErrInvalidStay = errors.New("stay must be between one and thirty nights")

// Stay is a half-open hotel stay: check-in is included and checkout is excluded.
type Stay struct {
	CheckIn  time.Time
	CheckOut time.Time
}

// NewStay validates the shared stay rules.
func NewStay(checkIn, checkOut time.Time) (Stay, error) {
	duration := checkOut.Sub(checkIn)
	nights := int(duration / (24 * time.Hour))
	if !checkOut.After(checkIn) || duration%(24*time.Hour) != 0 || nights > MaxStayNights {
		return Stay{}, ErrInvalidStay
	}
	return Stay{CheckIn: checkIn, CheckOut: checkOut}, nil
}

// Nights returns the number of occupied nights in the stay.
func (s Stay) Nights() int {
	return int(s.CheckOut.Sub(s.CheckIn).Hours() / 24)
}
