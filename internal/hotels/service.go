// Package hotels provides hotel discovery use cases.
package hotels

import (
	"context"
	"fmt"
	"strings"

	"github.com/chokosabe/hotel-bookings/internal/domain"
	"github.com/chokosabe/hotel-bookings/internal/persistence"
	"gorm.io/gorm"
)

// Service provides hotel-related application operations.
type Service struct {
	db *gorm.DB
}

// NewService constructs a hotel service backed by db.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Search finds hotels whose name contains name, case-insensitively.
func (s *Service) Search(ctx context.Context, name string) ([]domain.Hotel, error) {
	var rows []persistence.Hotel
	if err := s.db.WithContext(ctx).
		Where("lower(name) LIKE ?", "%"+strings.ToLower(strings.TrimSpace(name))+"%").
		Order("name, id").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("search hotels: %w", err)
	}

	hotels := make([]domain.Hotel, len(rows))
	for i, hotel := range rows {
		hotels[i] = persistence.HotelToDomain(hotel)
	}
	return hotels, nil
}
