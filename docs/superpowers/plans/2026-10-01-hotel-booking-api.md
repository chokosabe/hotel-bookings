# Hotel Booking API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a documented, Docker-runnable Go/Gin REST API that finds hotels, reports suitable room availability, creates non-overlapping bookings, and retrieves bookings.

**Architecture:** A Gin delivery layer validates HTTP input and delegates to focused application services. `database/sql` and a SQLite schema hold hotels, rooms, bookings, and a materialized `booking_nights` record per occupied night; the unique `(room_id, stay_date)` constraint makes double booking impossible inside a transaction. A notifier interface decouples confirmation-email simulation from booking creation and supports deterministic tests.

**Tech Stack:** Go 1.25, Gin, `database/sql`, `modernc.org/sqlite` (pure Go SQLite driver), Docker/Compose, OpenAPI 3.1, Go standard-library test tools.

## Global Constraints

- Use Gin; do not introduce an ORM, router library, configuration framework, or CI workflow.
- Use SQLite at `DATABASE_PATH`, defaulting to `./data/hotel-bookings.db`; run embedded SQL migrations at startup.
- Use `PORT`, default `8080`; use `ENABLE_TEST_ENDPOINTS`, default `true`; use structured JSON logs at INFO.
- Public API is JSON, snake_case, versioned below `/api/v1`; booking JSON must be `application/json` and reject unknown fields.
- Dates are ISO-8601 calendar dates and stays are half-open: `check_in` inclusive and `check_out` exclusive; maximum stay is 30 nights.
- The seed inventory is one hotel, “The Grand Hotel”: two single rooms (capacity 1), two double rooms (capacity 2), and two deluxe rooms (capacity 4).
- Assign the lowest adequate available capacity, then lowest room number. Guest count must be positive and no greater than the assigned room capacity.
- Create opaque `HBK-` booking references with a database uniqueness constraint. The database ID is not public.
- Keep seed/reset unauthenticated only while `ENABLE_TEST_ENDPOINTS=true`; resetting removes all hotel data and seed recreates it deterministically.
- Confirmation simulation starts only after commit, logs after two seconds, and is waited/cancelled as part of graceful shutdown.
- Explicit non-goals: authentication, cancellation/change flows, payments, pricing, room/hotel administration, and a rendered Swagger UI.
- Every task is TDD-first and ends with `make fmt-check vet test`; commit and push the independently runnable slice directly to branch `initial`.

---

## File Structure

- `cmd/api/main.go` — process composition, signal-aware server startup and shutdown.
- `internal/config/config.go` — environment parsing with defaults and validation.
- `internal/database/database.go` — SQLite opening, foreign-key setup, embedded migration application, transaction helper.
- `internal/database/migrations/001_initial.sql` — relational schema and database invariants.
- `internal/domain/*.go` — hotel, room, booking, validation/error types; no Gin or SQL types.
- `internal/hotels/service.go` — name search and deterministic availability selection.
- `internal/bookings/service.go` — atomic booking creation and reference lookup.
- `internal/notifications/notifier.go` — lifecycle-aware asynchronous confirmation abstraction.
- `internal/httpapi/*.go` — Gin routes, request decoding, response DTOs, shared error translation.
- `internal/evaluatordata/service.go` — deterministic reset/seed application service.
- `internal/*/*_test.go` — focused service and real-SQLite HTTP integration tests.
- `docs/PRD.md` and `docs/ARCHITECTURE.md` — decision records plus clearly marked author-completion sections.
- `docs/openapi.yaml` and `requests.http` — API contract and executable manual requests.
- `Makefile`, `.env.example`, `Dockerfile`, `docker-compose.yml`, `.dockerignore` — repeatable local operation.

### Task 1: Runnable Foundation, Schema, and Documentation

**Files:**
- Create: `go.mod`, `cmd/api/main.go`, `internal/config/config.go`, `internal/config/config_test.go`
- Create: `internal/database/database.go`, `internal/database/migrations/001_initial.sql`
- Create: `internal/httpapi/server.go`, `internal/httpapi/server_test.go`
- Create: `docs/PRD.md`, `docs/ARCHITECTURE.md`, `docs/openapi.yaml`, `requests.http`
- Create: `Makefile`, `.env.example`, `.dockerignore`, `Dockerfile`, `docker-compose.yml`
- Modify: `README.md`, `.gitignore`

**Interfaces:**
- Produces `config.Load(getenv func(string) string) (Config, error)` and `database.Open(ctx context.Context, path string) (*sql.DB, error)`.
- Produces `httpapi.NewHandler() http.Handler` with `GET /healthz` returning `200 {"status":"ok"}`.
- Produces schema tables `hotels`, `rooms`, `bookings`, and `booking_nights` for later services.

- [ ] **Step 1: Write failing configuration and health tests**

```go
func TestLoadUsesDefaults(t *testing.T) {
    cfg, err := config.Load(func(string) string { return "" })
    require.NoError(t, err)
    assert.Equal(t, "8080", cfg.Port)
    assert.Equal(t, "./data/hotel-bookings.db", cfg.DatabasePath)
    assert.True(t, cfg.EnableTestEndpoints)
}

func TestHealthz(t *testing.T) {
    recorder := httptest.NewRecorder()
    httpapi.NewHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
    assert.Equal(t, http.StatusOK, recorder.Code)
    assert.JSONEq(t, `{"status":"ok"}`, recorder.Body.String())
}
```

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `go test ./internal/config ./internal/httpapi`

Expected: FAIL because packages and `Load`/`NewHandler` do not exist.

- [ ] **Step 3: Implement the minimal foundation**

```go
// internal/config/config.go
package config

type Config struct { Port, DatabasePath string; EnableTestEndpoints bool }
func Load(getenv func(string) string) (Config, error) {
    cfg := Config{Port: getenv("PORT"), DatabasePath: getenv("DATABASE_PATH"), EnableTestEndpoints: getenv("ENABLE_TEST_ENDPOINTS") != "false"}
    if cfg.Port == "" { cfg.Port = "8080" }
    if cfg.DatabasePath == "" { cfg.DatabasePath = "./data/hotel-bookings.db" }
    return cfg, nil
}
```

```go
// internal/httpapi/server.go
func NewHandler() http.Handler {
    router := gin.New()
    router.Use(gin.Recovery())
    router.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
    return router
}
```

Implement `database.Open` using `modernc.org/sqlite`, execute `PRAGMA foreign_keys = ON`, and apply each embedded `*.sql` migration in filename order. The schema must include the following database protections:

```sql
CREATE TABLE hotels (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE);
CREATE TABLE rooms (
  id INTEGER PRIMARY KEY, hotel_id INTEGER NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
  number TEXT NOT NULL, type TEXT NOT NULL CHECK(type IN ('single','double','deluxe')),
  capacity INTEGER NOT NULL CHECK(capacity > 0), UNIQUE(hotel_id, number)
);
CREATE TABLE bookings (
  id INTEGER PRIMARY KEY, reference TEXT NOT NULL UNIQUE, room_id INTEGER NOT NULL REFERENCES rooms(id),
  check_in TEXT NOT NULL, check_out TEXT NOT NULL, guest_count INTEGER NOT NULL CHECK(guest_count > 0),
  lead_guest_name TEXT NOT NULL, lead_guest_email TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE booking_nights (
  booking_id INTEGER NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
  room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  stay_date TEXT NOT NULL, PRIMARY KEY(booking_id, stay_date), UNIQUE(room_id, stay_date)
);
```

`main.go` must load config, open/migrate the database, create an `http.Server`, and use `signal.NotifyContext` for SIGINT/SIGTERM. Add a five-second shutdown context. Add Make targets `fmt`, `fmt-check`, `vet`, `test`, `run`, `build`, `docker-up`, `docker-down`, and `check`; `check` invokes `fmt-check vet test`.

- [ ] **Step 4: Add operational and document artifacts**

Write `Dockerfile` as a multi-stage Go build with a non-root runtime user and a `./hotel-bookings` entrypoint. Write Compose with an `api` service, port `8080:8080`, `DATABASE_PATH=/data/hotel-bookings.db`, `ENABLE_TEST_ENDPOINTS=true`, and named `hotel-data:/data` volume. Put `data/` and `bin/` in `.gitignore`.

Write `docs/PRD.md` and `docs/ARCHITECTURE.md` with: purpose, in-scope requirements, explicit non-goals, all settled decisions/rationales above, assumptions, invariants, risks/trade-offs, and an `## Author completion` section for the author’s product/architecture narrative. The PRD must explain why constrained scope and deterministic inventory make the exercise demonstrable. The architecture doc must explain why Gin, SQLite, per-night rows, and the notifier boundary fit the intended outcome.

Write a README with prerequisites; `make run`, `make check`, and Docker usage; configuration table; links to both docs, OpenAPI, and requests; no-auth/test-endpoint warning; and a factual AI disclosure stating that AI assisted with scaffolding, tests, and documentation while the author reviewed decisions, implementation, and final submission.

Create the initial OpenAPI document describing `/healthz` and the later route placeholders as the contract will expand. Add a `requests.http` file setting `@baseUrl = http://localhost:8080` and a health request.

- [ ] **Step 5: Run the foundation verification**

Run: `make check && docker compose config`

Expected: formatting, vet, unit tests pass; Compose parses successfully.

- [ ] **Step 6: Commit and push the foundation**

Run:

```bash
git add .
git commit -m "chore: establish runnable API foundation"
git push origin initial
```

Expected: a clean working tree and the foundation commit on GitHub branch `initial`.

### Task 2: Deterministic Test Data and Hotel Search

**Files:**
- Create: `internal/domain/hotel.go`, `internal/hotels/service.go`, `internal/evaluatordata/service.go`
- Create: `internal/hotels/service_test.go`, `internal/httpapi/hotels_test.go`
- Modify: `internal/httpapi/server.go`, `cmd/api/main.go`, `docs/openapi.yaml`, `requests.http`, `README.md`

**Interfaces:**
- Consumes `*sql.DB` migrated in Task 1.
- Produces `evaluatordata.Service.Reset(ctx)` and `Seed(ctx)`; `hotels.Service.Search(ctx, name string) ([]domain.Hotel, error)`.
- Produces `POST /api/v1/test/reset`, `POST /api/v1/test/seed`, and `GET /api/v1/hotels?name={name}`.

- [ ] **Step 1: Write failing test-data and search tests**

```go
func TestSeedIsIdempotentAndCreatesSixRooms(t *testing.T) {
    svc := newTestDataService(t)
    require.NoError(t, svc.Seed(context.Background()))
    require.NoError(t, svc.Seed(context.Background()))
    assertHotelRoomCounts(t, testDB(t), "The Grand Hotel", map[string]int{"single": 2, "double": 2, "deluxe": 2})
}

func TestHotelSearchIsCaseInsensitiveSubstring(t *testing.T) {
    seed(t)
    response := get(t, "/api/v1/hotels?name=GRAND")
    assert.Equal(t, 200, response.Code)
    assert.JSONEq(t, `[{"id":1,"name":"The Grand Hotel"}]`, response.Body.String())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/evaluatordata ./internal/hotels ./internal/httpapi`

Expected: FAIL because services and routes do not exist.

- [ ] **Step 3: Implement test data and hotel lookup**

Define `domain.Hotel{ID int64; Name string}` and `domain.Room{ID, HotelID int64; Number, Type string; Capacity int}`. `Reset` must run `DELETE FROM hotels` in a transaction. `Seed` must transactionally `INSERT OR IGNORE` “The Grand Hotel” then insert exactly `101/102 single 1`, `201/202 double 2`, and `301/302 deluxe 4` with `INSERT OR IGNORE`.

Implement search with `WHERE lower(name) LIKE '%' || lower(?) || '%' ORDER BY name, id`. Make `name` mandatory. The handler returns `400` for missing/blank name and `200 []` for no match. Mount test routes only when `EnableTestEndpoints` is true; if disabled they must be absent (`404`). Return all failures through `{"error":{"code":"...","message":"..."}}`.

- [ ] **Step 4: Update API documentation and examples**

Add exact seed/reset and hotel-search schemas, status codes, and error response schemas to `docs/openapi.yaml`. Add runnable calls to `requests.http` in this order: reset, seed, search successful, search no match. Update the README endpoint summary and setup workflow.

- [ ] **Step 5: Run verification**

Run: `make check && go run ./cmd/api & pid=$!; sleep 1; curl -fsS -X POST localhost:8080/api/v1/test/reset; curl -fsS -X POST localhost:8080/api/v1/test/seed; curl -fsS 'localhost:8080/api/v1/hotels?name=grand'; kill $pid`

Expected: checks pass; curl prints the seeded hotel JSON.

- [ ] **Step 6: Commit and push**

```bash
git add .
git commit -m "feat: add deterministic hotel test data and search"
git push origin initial
```

### Task 3: Suitable Room Availability

**Files:**
- Create: `internal/hotels/availability.go`, `internal/hotels/availability_test.go`
- Modify: `internal/httpapi/server.go`, `internal/httpapi/hotels_test.go`, `docs/openapi.yaml`, `requests.http`, `README.md`

**Interfaces:**
- Consumes seeded rooms and booking-night data from prior tasks.
- Produces `hotels.Service.AvailableRooms(ctx, hotelID int64, checkIn, checkOut time.Time, guests int) ([]domain.Room, error)` and `GET /api/v1/hotels/{hotelID}/availability`.

- [ ] **Step 1: Write failing availability tests**

```go
func TestAvailabilityExcludesRoomWithOccupiedNight(t *testing.T) {
    seed(t)
    createBooking(t, room101, "2026-12-10", "2026-12-12", 1)
    rooms := available(t, hotelID, "2026-12-10", "2026-12-12", 1)
    assert.NotContains(t, roomNumbers(rooms), "101")
}

func TestAvailabilityAllowsCheckoutDateReuseAndOrdersLowestAdequateCapacity(t *testing.T) {
    seed(t)
    createBooking(t, room101, "2026-12-10", "2026-12-12", 1)
    rooms := available(t, hotelID, "2026-12-12", "2026-12-13", 1)
    assert.Equal(t, []string{"101", "102", "201", "202", "301", "302"}, roomNumbers(rooms))
}

func TestAvailabilityRejectsInvalidStayAndGuestCount(t *testing.T) {
    response := get(t, "/api/v1/hotels/1/availability?check_in=2026-12-12&check_out=2026-12-12&guests=0")
    assert.Equal(t, http.StatusBadRequest, response.Code)
}
```

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `go test ./internal/hotels ./internal/httpapi`

Expected: FAIL because availability service and route do not exist.

- [ ] **Step 3: Implement date validation and availability query**

Use `time.Parse("2006-01-02", value)`. Require `check_in < check_out`, a maximum 30-night interval, and integer `guests >= 1`. Confirm hotel existence first; return a typed not-found error if absent. Query rooms with `capacity >= ?` and `NOT EXISTS` booking nights in `[check_in, check_out)`, ordered `capacity ASC, CAST(number AS INTEGER) ASC, number ASC`. Return room ID, number, type, and capacity. Invalid input is a `400`; unknown hotel is `404`; zero free rooms is `200 []`.

- [ ] **Step 4: Update contract and executable examples**

Document all availability parameters, examples, response fields, and `400`/`404` results in OpenAPI. Add valid, empty-result, invalid-date, and unknown-hotel availability requests to `requests.http`. Update README endpoint summary.

- [ ] **Step 5: Run verification**

Run: `make check`

Expected: all tests, format verification, and vet pass.

- [ ] **Step 6: Commit and push**

```bash
git add .
git commit -m "feat: expose suitable room availability"
git push origin initial
```

### Task 4: Atomic Booking Creation and Confirmation Notification

**Files:**
- Create: `internal/domain/booking.go`, `internal/bookings/service.go`, `internal/bookings/service_test.go`
- Create: `internal/notifications/notifier.go`, `internal/notifications/notifier_test.go`
- Create: `internal/httpapi/bookings_test.go`
- Modify: `internal/httpapi/server.go`, `cmd/api/main.go`, `docs/openapi.yaml`, `requests.http`, `README.md`, `docs/ARCHITECTURE.md`

**Interfaces:**
- Consumes `hotels.Service.AvailableRooms`, the schema constraints, and a `notifications.Notifier`.
- Produces `bookings.Service.Create(ctx context.Context, input CreateInput) (domain.Booking, error)` and `POST /api/v1/bookings`.
- Produces `Notifier.NotifyBookingConfirmed(context.Context, domain.Booking)` and `Notifier.Wait(context.Context) error`.

- [ ] **Step 1: Write failing booking and notifier tests**

```go
func TestCreateAssignsLowestAdequateRoomAndWritesEachNight(t *testing.T) {
    booking := create(t, CreateInput{HotelID: hotelID, CheckIn: "2026-12-10", CheckOut: "2026-12-12", GuestCount: 1, LeadGuestName: "Ada Lovelace", LeadGuestEmail: "ada@example.com"})
    assert.Equal(t, "101", booking.Room.Number)
    assert.Regexp(t, `^HBK-[A-Z0-9]{12}$`, booking.Reference)
    assertBookingNightCount(t, booking.ID, 2)
}

func TestConcurrentFinalRoomRequestsYieldOneCreatedAndOneConflict(t *testing.T) {
    // Prepare exactly one capacity-1 room; concurrently call Create twice for same night.
    statuses := concurrentCreateStatuses(t)
    assert.ElementsMatch(t, []int{http.StatusCreated, http.StatusConflict}, statuses)
}

func TestNotifierLogsOnlyAfterDelayAndWaitsForWork(t *testing.T) {
    notifier, logs := newTestNotifier(t)
    notifier.NotifyBookingConfirmed(context.Background(), booking)
    require.NoError(t, notifier.Wait(context.Background()))
    assert.Contains(t, logs.String(), booking.Reference)
}
```

- [ ] **Step 2: Run focused tests to verify they fail**

Run: `go test ./internal/bookings ./internal/notifications ./internal/httpapi`

Expected: FAIL because booking service, notifier, and route do not exist.

- [ ] **Step 3: Implement transactionally safe booking**

Define `CreateInput` with hotel ID, dates, guest count, lead guest name/email. Validate date rules, 1–30 nights, nonblank name, and `net/mail.ParseAddress` email before opening a transaction. Start a transaction, select the first available room by the Task 3 ordering, insert a `HBK-` plus 12 crypto-random uppercase base36 characters reference, insert the booking, then insert one `booking_nights` row for each date. If `booking_nights` unique constraint fails, roll back and return typed conflict; retry only a reference collision. Commit before calling notifier. There must be no notifier call on error or rollback.

Use a notifier implementation with a `sync.WaitGroup`, root cancellable context, injected `time.After`/clock seam for tests, and `slog.Logger`. Each notification starts one goroutine, waits two seconds unless cancelled, then logs booking reference and guest email. `Wait` blocks until pending work completes or context expires; `Close` cancels the root context. Wire `Close`/`Wait` into main’s five-second shutdown path.

The HTTP handler must require `Content-Type: application/json`, use `json.Decoder.DisallowUnknownFields`, reject trailing JSON, map validation to `400`, no suitable room/database occupancy conflict to `409`, and success to `201` with booking reference, room, dates, party size, and lead guest details.

- [ ] **Step 4: Update documentation**

Make booking request/response examples and all `400`/`409` contract details exact in OpenAPI and `requests.http`. Update README usage flow. Add to architecture docs an explanation that the `UNIQUE(room_id, stay_date)` constraint is the final concurrency protection and the notifier fires post-commit.

- [ ] **Step 5: Run concurrency and full checks**

Run: `go test -race ./... && make check`

Expected: race detector and all normal checks pass.

- [ ] **Step 6: Commit and push**

```bash
git add .
git commit -m "feat: create atomic bookings with confirmation notifications"
git push origin initial
```

### Task 5: Booking Lookup and Submission Hardening

**Files:**
- Create: `internal/bookings/lookup_test.go`
- Modify: `internal/bookings/service.go`, `internal/httpapi/server.go`, `internal/httpapi/bookings_test.go`
- Modify: `docs/openapi.yaml`, `requests.http`, `README.md`, `docs/PRD.md`, `docs/ARCHITECTURE.md`

**Interfaces:**
- Consumes `bookings.Service` and persisted booking/room data.
- Produces `bookings.Service.FindByReference(ctx context.Context, reference string) (domain.Booking, error)` and `GET /api/v1/bookings/{reference}`.

- [ ] **Step 1: Write failing lookup tests**

```go
func TestFindBookingByReferenceReturnsAssignedRoomAndGuestDetails(t *testing.T) {
    created := create(t, validInput())
    response := get(t, "/api/v1/bookings/"+created.Reference)
    assert.Equal(t, http.StatusOK, response.Code)
    assert.JSONEq(t, expectedBookingJSON(created), response.Body.String())
}

func TestFindBookingByUnknownReferenceReturnsNotFound(t *testing.T) {
    response := get(t, "/api/v1/bookings/HBK-DOESNOTEXIST")
    assert.Equal(t, http.StatusNotFound, response.Code)
    assert.JSONEq(t, `{"error":{"code":"booking_not_found","message":"booking not found"}}`, response.Body.String())
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/bookings ./internal/httpapi`

Expected: FAIL because lookup method and route do not exist.

- [ ] **Step 3: Implement lookup and final error consistency**

Join `bookings` to `rooms` and `hotels` by booking reference and return the same booking representation as creation. Map `sql.ErrNoRows` to typed booking-not-found. Ensure all public handlers use the shared structured error serializer and never leak driver errors. Preserve reference and date values exactly as stored.

- [ ] **Step 4: Complete API and design documentation**

Add lookup success/not-found definitions and request to OpenAPI and `requests.http`. Update README with a complete reset → seed → search → availability → book → lookup walkthrough. In PRD and architecture docs add a final “implemented scope” checklist, a decision table of assumptions and rationale, and author-completion prompts. Confirm documentation clearly states why no CI was added: the Makefile supplies local, reproducible review gates for this coding exercise.

- [ ] **Step 5: Run final acceptance verification**

Run:

```bash
make check
go test -race ./...
docker compose up --build -d
curl -fsS http://localhost:8080/healthz
docker compose down -v
```

Expected: all checks pass, container answers health check, and the Compose stack cleans up.

- [ ] **Step 6: Commit and push final slice**

```bash
git add .
git commit -m "feat: retrieve bookings by reference"
git push origin initial
```

## Self-Review

- **Spec coverage:** Task 2 covers finding hotels and required seed/reset operations. Task 3 covers date/person-filtered room availability. Task 4 covers room capacity, one-room-per-stay assignment, non-overlap, unique references, and delayed concurrent confirmation simulation. Task 5 covers booking-reference lookup. Task 1 provides Gin, SQLite, Docker/Compose, operational health/shutdown, documentation, AI disclosure, and local test gates. No authentication is introduced.
- **Placeholder scan:** Implementation tasks name concrete paths, contracts, schema constraints, commands, and expected results. The generated PRD/architecture templates intentionally include author-completion sections, which are a product requirement rather than unspecified implementation work.
- **Type consistency:** The plan consistently uses `Config`, `database.Open`, `domain.Hotel`, `domain.Room`, `domain.Booking`, `testdata.Service`, `hotels.Service`, `bookings.Service`, `CreateInput`, and `notifications.Notifier`. Booking output is shared between creation and lookup.
