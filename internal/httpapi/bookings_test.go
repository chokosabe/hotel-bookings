package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestCreateBookingEndpointAssignsRoomAndReturnsReference(t *testing.T) {
	handler := newFeatureHandler(t, true)
	if response := serve(handler, http.MethodPost, "/api/v1/test/seed"); response.Code != http.StatusNoContent {
		t.Fatalf("seed status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response := serveBooking(handler, `{
		"hotel_id": 1,
		"check_in": "2026-12-10",
		"check_out": "2026-12-12",
		"guest_count": 1,
		"lead_guest_name": "Ada Lovelace",
		"lead_guest_email": "ada@example.com"
	}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}

	var body struct {
		Reference string `json:"reference"`
		Room      struct {
			Number string `json:"number"`
		} `json:"room"`
		CheckIn  string `json:"check_in"`
		CheckOut string `json:"check_out"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !regexp.MustCompile(`^HBK-[A-Z2-7]{12}$`).MatchString(body.Reference) {
		t.Errorf("reference = %q, want HBK- plus 12 uppercase base32 characters", body.Reference)
	}
	if body.Room.Number != "101" {
		t.Errorf("room number = %q, want 101", body.Room.Number)
	}
	if body.CheckIn != "2026-12-10" || body.CheckOut != "2026-12-12" {
		t.Errorf("stay = %s to %s, want submitted dates", body.CheckIn, body.CheckOut)
	}
}

func TestFindBookingEndpointReturnsCreatedBooking(t *testing.T) {
	handler := newFeatureHandler(t, true)
	if response := serve(handler, http.MethodPost, "/api/v1/test/seed"); response.Code != http.StatusNoContent {
		t.Fatalf("seed status = %d, want %d", response.Code, http.StatusNoContent)
	}
	created := serveBooking(handler, `{"hotel_id":1,"check_in":"2026-12-10","check_out":"2026-12-12","guest_count":1,"lead_guest_name":"Ada Lovelace","lead_guest_email":"ada@example.com"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", created.Code, http.StatusCreated)
	}
	var createBody struct {
		Reference string `json:"reference"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createBody); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	found := serve(handler, http.MethodGet, "/api/v1/bookings/"+createBody.Reference)
	if found.Code != http.StatusOK {
		t.Fatalf("lookup status = %d, want %d: %s", found.Code, http.StatusOK, found.Body.String())
	}
	var foundBody struct {
		Reference string `json:"reference"`
		Room      struct {
			Number string `json:"number"`
		} `json:"room"`
	}
	if err := json.Unmarshal(found.Body.Bytes(), &foundBody); err != nil {
		t.Fatalf("decode lookup response: %v", err)
	}
	if foundBody.Reference != createBody.Reference || foundBody.Room.Number != "101" {
		t.Errorf("lookup body = %#v, want reference and room 101", foundBody)
	}

	missing := serve(handler, http.MethodGet, "/api/v1/bookings/HBK-NOTFOUND")
	if missing.Code != http.StatusNotFound {
		t.Errorf("missing lookup status = %d, want %d", missing.Code, http.StatusNotFound)
	}
}

func TestCreateBookingEndpointRejectsInvalidJSONAndUnavailableRoom(t *testing.T) {
	handler := newFeatureHandler(t, true)
	if response := serve(handler, http.MethodPost, "/api/v1/test/seed"); response.Code != http.StatusNoContent {
		t.Fatalf("seed status = %d, want %d", response.Code, http.StatusNoContent)
	}

	invalid := serveBooking(handler, `{"hotel_id":1,"hotel_id":1}`)
	if invalid.Code != http.StatusBadRequest {
		t.Errorf("duplicate-field status = %d, want %d", invalid.Code, http.StatusBadRequest)
	}
	unknown := serveBooking(handler, `{"unexpected": true}`)
	if unknown.Code != http.StatusBadRequest {
		t.Errorf("unknown-field status = %d, want %d", unknown.Code, http.StatusBadRequest)
	}
	noContentTypeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewBufferString(`{}`))
	noContentTypeResponse := httptest.NewRecorder()
	handler.ServeHTTP(noContentTypeResponse, noContentTypeRequest)
	if noContentTypeResponse.Code != http.StatusBadRequest {
		t.Errorf("missing content type status = %d, want %d", noContentTypeResponse.Code, http.StatusBadRequest)
	}

	unavailable := serveBooking(handler, `{
		"hotel_id": 1,
		"check_in": "2026-12-10",
		"check_out": "2026-12-12",
		"guest_count": 5,
		"lead_guest_name": "Ada Lovelace",
		"lead_guest_email": "ada@example.com"
	}`)
	if unavailable.Code != http.StatusConflict {
		t.Errorf("unavailable status = %d, want %d: %s", unavailable.Code, http.StatusConflict, unavailable.Body.String())
	}
}

func serveBooking(handler http.Handler, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
