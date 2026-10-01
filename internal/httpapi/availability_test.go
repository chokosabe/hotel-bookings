package httpapi_test

import (
	"net/http"
	"testing"
)

func TestAvailabilityEndpointReturnsLowestAdequateRooms(t *testing.T) {
	handler := newFeatureHandler(t, true)
	if response := serve(handler, http.MethodPost, "/api/v1/test/seed"); response.Code != http.StatusNoContent {
		t.Fatalf("seed status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response := serve(handler, http.MethodGet, "/api/v1/hotels/1/availability?check_in=2026-12-10&check_out=2026-12-12&guests=3")
	if response.Code != http.StatusOK {
		t.Fatalf("availability status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	want := `[{"id":5,"number":"301","type":"deluxe","capacity":4},{"id":6,"number":"302","type":"deluxe","capacity":4}]`
	if got := response.Body.String(); got != want {
		t.Errorf("availability body = %s, want %s", got, want)
	}

	empty := serve(handler, http.MethodGet, "/api/v1/hotels/1/availability?check_in=2026-12-10&check_out=2026-12-12&guests=5")
	if empty.Code != http.StatusOK || empty.Body.String() != "[]" {
		t.Errorf("empty availability = %d %s, want 200 []", empty.Code, empty.Body.String())
	}
}

func TestAvailabilityEndpointValidatesInputAndMissingHotel(t *testing.T) {
	handler := newFeatureHandler(t, true)

	invalid := serve(handler, http.MethodGet, "/api/v1/hotels/1/availability?check_in=2026-12-12&check_out=2026-12-12&guests=0")
	if invalid.Code != http.StatusBadRequest {
		t.Errorf("invalid availability status = %d, want %d", invalid.Code, http.StatusBadRequest)
	}

	missing := serve(handler, http.MethodGet, "/api/v1/hotels/999/availability?check_in=2026-12-10&check_out=2026-12-11&guests=1")
	if missing.Code != http.StatusNotFound {
		t.Errorf("missing hotel status = %d, want %d", missing.Code, http.StatusNotFound)
	}
}
