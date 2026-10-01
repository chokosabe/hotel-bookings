package hotels_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/chokosabe/hotel-bookings/internal/database"
	"github.com/chokosabe/hotel-bookings/internal/evaluatordata"
	"github.com/chokosabe/hotel-bookings/internal/hotels"
)

func TestSearchMatchesCaseInsensitiveSubstring(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := evaluatordata.NewService(db).Seed(context.Background()); err != nil {
		t.Fatalf("seed database: %v", err)
	}

	found, err := hotels.NewService(db).Search(context.Background(), "GRAND")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("Search() returned %d hotels, want 1", len(found))
	}
	if found[0].Name != "The Grand Hotel" {
		t.Errorf("hotel name = %q, want The Grand Hotel", found[0].Name)
	}
}
