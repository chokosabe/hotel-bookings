package domain

import "errors"

// ErrHotelNotFound means a requested hotel identifier does not exist.
var ErrHotelNotFound = errors.New("hotel not found")
