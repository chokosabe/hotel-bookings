// Package hotels provides hotel discovery use cases.
package hotels

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/chokosabe/hotel-bookings/internal/domain"
)

// Service provides hotel-related application operations.
type Service struct {
	db *sql.DB
}

// NewService constructs a hotel service backed by db.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Search finds hotels whose name contains name, case-insensitively.
func (s *Service) Search(ctx context.Context, name string) ([]domain.Hotel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name
		FROM hotels
		WHERE lower(name) LIKE '%' || lower(?) || '%'
		ORDER BY name, id
	`, strings.TrimSpace(name))
	if err != nil {
		return nil, fmt.Errorf("search hotels: %w", err)
	}
	defer rows.Close()

	hotels := make([]domain.Hotel, 0)
	for rows.Next() {
		var hotel domain.Hotel
		if err := rows.Scan(&hotel.ID, &hotel.Name); err != nil {
			return nil, fmt.Errorf("scan hotel: %w", err)
		}
		hotels = append(hotels, hotel)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hotels: %w", err)
	}
	return hotels, nil
}
