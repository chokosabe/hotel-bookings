# GORM Persistence Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hand-written `database/sql` persistence implementation with GORM while preserving SQLite, all API behaviour, database invariants, Docker portability, and the existing public contract.

**Architecture:** The application services will receive `*gorm.DB` and retain their current domain-facing methods. A small `internal/persistence` package will hold GORM-only row models and conversion helpers so `internal/domain` stays persistence-independent. GORM's transaction and query-builder APIs will own schema creation, seed/reset, lookup, availability, and booking writes; no services will manually scan SQL rows or call `database/sql` transactions.

**Tech Stack:** Go 1.25, Gin, GORM, `github.com/glebarez/sqlite` (pure-Go GORM SQLite dialect backed by modernc SQLite), Docker/Compose, Go standard-library test tools.

## Global Constraints

- Retain SQLite, `DATABASE_PATH`, one physical connection, foreign keys, and five-second busy timeout.
- Use GORM's `AutoMigrate` to create `hotels`, `rooms`, `bookings`, and `booking_nights`; remove embedded SQL migrations and the migration ledger.
- Use `github.com/glebarez/sqlite`, not the CGO-based official SQLite GORM driver, so the existing `CGO_ENABLED=0` Docker build stays portable.
- Preserve table names and column names exactly so existing SQLite files remain readable; existing records must not be deleted or rewritten by startup migration.
- Preserve database constraints: unique hotel name; room type/capacity checks; unique `(hotel_id, number)`; unique booking reference; positive guest count; foreign keys; composite booking-night primary key; unique `(room_id, stay_date)`.
- Keep domain types free of GORM tags, service API signatures and HTTP responses unchanged, and continue to invoke notification only after transaction commit.
- Avoid service-level raw `Query`, `QueryRow`, `Scan`, `Exec`, and `BeginTx` calls. GORM query expressions/subqueries are permitted where SQL `NOT EXISTS` is needed.
- Continue to validate with `make check`, `go test -race ./...`, `golangci-lint`, manual API smoke testing, Docker build, and a clean working tree before every commit/push to `initial`.

---

## File Structure

- `internal/database/database.go` — opens/configures `*gorm.DB`, applies `AutoMigrate`, configures pool.
- `internal/persistence/models.go` — GORM models, explicit table names, relations, constraints, and conversion helpers.
- `internal/persistence/models_test.go` — migration schema/invariant tests through GORM.
- `internal/evaluatordata/service.go` — GORM seed/reset transaction.
- `internal/hotels/service.go` / `availability.go` — GORM search and availability queries.
- `internal/bookings/service.go` — GORM transaction, query, creation, and lookup.
- `cmd/api/main.go` and all service/handler test fixtures — use `*gorm.DB` supplied by `database.Open`.
- `go.mod`, `go.sum`, `docs/ARCHITECTURE.md`, `README.md` — dependencies and decision record.
- Delete: `internal/database/migrations/001_initial.sql`.

### Task 1: Establish the GORM SQLite Adapter and Compatibility Models

**Files:**
- Create: `internal/persistence/models.go`, `internal/persistence/models_test.go`
- Modify: `internal/database/database.go`, `internal/database/database_test.go`, `go.mod`, `go.sum`, `docs/ARCHITECTURE.md`, `README.md`
- Delete: `internal/database/migrations/001_initial.sql`

**Interfaces:**
- Produces `database.Open(ctx context.Context, path string) (*gorm.DB, error)`.
- Produces persistence-only models `Hotel`, `Room`, `Booking`, and `BookingNight`; each implements an explicit `TableName() string`.
- Consumes current SQLite file table/column names and produces the same relational database contract through `AutoMigrate`.

- [ ] **Step 1: Add failing migration compatibility tests**

```go
func TestOpenAutoMigratesDomainTablesAndEnforcesNightUniqueness(t *testing.T) {
    db := openTestDatabase(t)
    requireTable(t, db, "hotels", "rooms", "bookings", "booking_nights")
    room := createRoom(t, db)
    booking := createBooking(t, db, room.ID)
    require.NoError(t, db.Create(&persistence.BookingNight{BookingID: booking.ID, RoomID: room.ID, StayDate: "2026-12-10"}).Error)
    err := db.Create(&persistence.BookingNight{BookingID: booking.ID + 1, RoomID: room.ID, StayDate: "2026-12-10"}).Error
    if err == nil { t.Fatal("duplicate room-night unexpectedly succeeded") }
}
```

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `go test ./internal/database ./internal/persistence`

Expected: FAIL because `database.Open` returns `*sql.DB`, and no GORM models exist.

- [ ] **Step 3: Add GORM dependencies and persistence models**

Run:

```bash
go get gorm.io/gorm github.com/glebarez/sqlite
go mod tidy
```

Create explicit GORM row models. The essential constraints must be represented as tags:

```go
type Room struct {
    ID       int64  `gorm:"primaryKey"`
    HotelID  int64  `gorm:"not null;uniqueIndex:idx_rooms_hotel_number"`
    Number   string `gorm:"not null;uniqueIndex:idx_rooms_hotel_number"`
    Type     string `gorm:"not null;check:chk_rooms_type,type IN ('single','double','deluxe')"`
    Capacity int    `gorm:"not null;check:chk_rooms_capacity,capacity > 0"`
}
func (Room) TableName() string { return "rooms" }

type BookingNight struct {
    BookingID int64  `gorm:"primaryKey"`
    RoomID    int64  `gorm:"not null;uniqueIndex:idx_booking_nights_room_date"`
    StayDate  string `gorm:"primaryKey;uniqueIndex:idx_booking_nights_room_date"`
}
func (BookingNight) TableName() string { return "booking_nights" }
```

Give `Hotel`, `Room`, and `Booking` real relation fields and `constraint:OnDelete:CASCADE` on the relationships that currently cascade. Keep dates and `CreatedAt` as strings in persistence models so pre-existing values remain byte-compatible; conversion helpers parse them into domain values only at service boundaries.

- [ ] **Step 4: Replace database opening and migration**

Open the pure-Go GORM SQLite dialector with a DSN that applies `foreign_keys(1)` and `busy_timeout(5000)`. Configure `TranslateError: true`, acquire `sqlDB, err := db.DB()`, set max open/idle connections to one, then call:

```go
if err := db.WithContext(ctx).AutoMigrate(
    &persistence.Hotel{}, &persistence.Room{}, &persistence.Booking{}, &persistence.BookingNight{},
); err != nil {
    return nil, fmt.Errorf("auto-migrate database: %w", err)
}
```

Remove `embed`, migration discovery, and `schema_migrations`; retain parent-directory creation. Update tests to use `db.Migrator().HasTable` and GORM `Create`/`Count`, not `database/sql` operations.

- [ ] **Step 5: Document the decision**

Update architecture documentation to explain the GORM boundary, pure-Go SQLite dialect choice, `AutoMigrate` trade-off (appropriate for this small controlled schema but needs reviewed migrations for complex production changes), and compatibility contract. Update README implementation notes without changing public usage.

- [ ] **Step 6: Verify and commit the foundation slice**

Run:

```bash
make check
go test -race ./...
go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run ./...
docker build -t hotel-bookings:gorm-foundation .
```

Expected: all checks pass. Then commit and push:

```bash
git add .
git commit -m "refactor: establish GORM SQLite persistence"
git push origin initial
```

### Task 2: Migrate Seed, Hotel, and Availability Services to GORM

**Files:**
- Modify: `internal/evaluatordata/service.go`, `internal/evaluatordata/service_test.go`
- Modify: `internal/hotels/service.go`, `internal/hotels/availability.go`, `internal/hotels/*_test.go`
- Modify: `cmd/api/main.go`, `internal/httpapi/*_test.go`

**Interfaces:**
- Consumes `*gorm.DB` from Task 1 and persistence models/converters.
- Preserves `evaluatordata.NewService(db)`, `hotels.NewService(db)`, `Search`, and `AvailableRooms` signatures and results exactly.

- [ ] **Step 1: Update tests to construct GORM test databases**

Replace all test fixture references to `*sql.DB` and direct SQL setup with `*gorm.DB` and persistence-model creation. Keep the existing assertions for seeded inventory, case-insensitive matching, availability ordering, occupancy exclusion, checkout-date reuse, empty results, and disabled test routes.

- [ ] **Step 2: Run tests to verify the old service implementations no longer compile**

Run: `go test ./internal/evaluatordata ./internal/hotels ./internal/httpapi`

Expected: FAIL on incompatible `*sql.DB` service fields/constructors.

- [ ] **Step 3: Replace service persistence calls with GORM**

Use `db.WithContext(ctx).Transaction` for seed. Use `FirstOrCreate` for “The Grand Hotel” and `Clauses(clause.OnConflict{DoNothing: true}).Create(&rooms)` for idempotent rooms. Use `db.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate:true}).Delete(&persistence.Hotel{})` for reset.

Use GORM `Where("lower(name) LIKE ?", "%"+strings.ToLower(name)+"%")`, `Order("name, id")`, and `Find` for search. For availability, first use `First` to distinguish a missing hotel, then express the occupied-night exclusion as a GORM subquery:

```go
occupied := db.Model(&persistence.BookingNight{}).Select("1").
    Where("booking_nights.room_id = rooms.id").
    Where("stay_date >= ? AND stay_date < ?", checkIn, checkOut)
err := db.WithContext(ctx).Where("hotel_id = ? AND capacity >= ?", hotelID, guests).
    Where("NOT EXISTS (?)", occupied).
    Order("capacity ASC").Order("CAST(number AS INTEGER) ASC").Order("number ASC").Find(&rows).Error
```

Convert persistence rows to existing domain structs before returning.

- [ ] **Step 4: Verify and commit the service slice**

Run: `make check && go test -race ./... && go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run ./...`

Then:

```bash
git add .
git commit -m "refactor: migrate hotel services to GORM"
git push origin initial
```

### Task 3: Migrate Booking Transactions and Lookup to GORM

**Files:**
- Modify: `internal/bookings/service.go`, `internal/bookings/service_test.go`, `internal/httpapi/*_test.go`
- Modify: `cmd/api/main.go`, `docs/ARCHITECTURE.md`, `README.md`

**Interfaces:**
- Consumes Task 1 models and GORM database and preserves `bookings.NewService`, `Create`, `FindByReference`, domain errors, notifier behavior, and HTTP status/JSON contract.

- [ ] **Step 1: Keep and extend behavioural tests before replacing implementation**

Retain tests that prove lowest adequate assignment, exact booking-night count, notification post-success only, invalid booking rejection, one winner under concurrent final-room requests, lookup success, and lookup-not-found. Add an assertion that `ErrNoSuitableRoom` results from a GORM translated duplicate-key error or equivalent SQLite unique violation when a room-night race is forced.

- [ ] **Step 2: Run booking tests to verify incompatibility before implementation**

Run: `go test ./internal/bookings ./internal/httpapi`

Expected: FAIL because booking service still requires `*sql.DB`/`*sql.Tx`.

- [ ] **Step 3: Implement GORM transaction and lookup**

Use `db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { ... })`. Check hotel existence with `First`, select the candidate using the same GORM `NOT EXISTS` subquery and order as Task 2, create the booking model, create each `BookingNight`, and map GORM duplicate-key errors to `ErrNoSuitableRoom`. Return the complete domain booking only after `Transaction` succeeds, then call `notifier.NotifyBookingConfirmed`.

For lookup, use `Joins("Room")` (or a scoped explicit GORM join), `Where("bookings.reference = ?", strings.TrimSpace(reference))`, and map `gorm.ErrRecordNotFound` to `ErrBookingNotFound`; convert the result to the current domain booking response.

- [ ] **Step 4: Final regression verification and commit**

Run:

```bash
make check
go test -race ./...
go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run ./...
docker build -t hotel-bookings:gorm-final .
```

Run the executable on an isolated temporary SQLite path; seed it, create a booking, retrieve it by returned reference, and confirm the delayed notification is logged. Then:

```bash
git add .
git commit -m "refactor: migrate booking persistence to GORM"
git push origin initial
```

## Self-Review

- **Spec coverage:** The plan retains every API feature, capacity and one-room rules, unique references, no-double-booking database invariant, seed/reset endpoints, asynchronous confirmation, no-auth scope, tests, documentation, Docker, and Makefile workflow.
- **Compatibility:** Explicit table/column names, strings for stored dates/timestamps, and constraints preserve the existing SQLite contract. AutoMigrate is additive for current database files; tests must exercise both fresh and pre-existing schema paths.
- **No hidden database/sql dependency:** Each task removes it from production services. `database/sql` is used only indirectly by GORM to set the connection pool.
- **Risk control:** The highest-risk paths—per-night uniqueness and transaction ordering—remain covered by real SQLite, concurrent service tests, race detection, and the linter before each push.
