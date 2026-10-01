package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/chokosabe/hotel-bookings/internal/bookings"
	"github.com/chokosabe/hotel-bookings/internal/database"
	"github.com/chokosabe/hotel-bookings/internal/evaluatordata"
	"github.com/chokosabe/hotel-bookings/internal/hotels"
	"github.com/chokosabe/hotel-bookings/internal/httpapi"
)

func TestTestDataAndHotelSearchEndpoints(t *testing.T) {
	handler := newFeatureHandler(t, true)

	for _, path := range []string{"/api/v1/test/reset", "/api/v1/test/seed", "/api/v1/test/seed"} {
		response := serve(handler, http.MethodPost, path)
		if response.Code != http.StatusNoContent {
			t.Fatalf("POST %s status = %d, want %d: %s", path, response.Code, http.StatusNoContent, response.Body.String())
		}
	}

	response := serve(handler, http.MethodGet, "/api/v1/hotels?name=GRAND")
	if response.Code != http.StatusOK {
		t.Fatalf("search status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if got, want := response.Body.String(), `[{"id":1,"name":"The Grand Hotel"}]`; got != want {
		t.Errorf("search body = %s, want %s", got, want)
	}

	response = serve(handler, http.MethodGet, "/api/v1/hotels?name=missing")
	if response.Code != http.StatusOK || response.Body.String() != "[]" {
		t.Errorf("no-match response = %d %s, want 200 []", response.Code, response.Body.String())
	}
}

func TestSearchRequiresName(t *testing.T) {
	response := serve(newFeatureHandler(t, true), http.MethodGet, "/api/v1/hotels")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if got, want := response.Body.String(), `{"error":{"code":"invalid_request","message":"name query parameter is required"}}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestTestEndpointsCanBeDisabled(t *testing.T) {
	response := serve(newFeatureHandler(t, false), http.MethodPost, "/api/v1/test/seed")
	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func newFeatureHandler(t *testing.T, enableTestEndpoints bool) http.Handler {
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

	return httpapi.NewHandler(httpapi.Dependencies{
		Bookings:            bookings.NewService(db, nil),
		Hotels:              hotels.NewService(db),
		TestData:            evaluatordata.NewService(db),
		EnableTestEndpoints: enableTestEndpoints,
	})
}

func serve(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, nil))
	return response
}
