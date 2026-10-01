# Hotel Bookings API — Architecture

## Overview

The API is a single Go process with three layers. Gin handlers parse requests and encode responses. Application services own hotel search, availability, test data, and booking decisions. GORM maps those services onto SQLite. Keeping HTTP out of the services means the booking rules are tested directly against a real, temporary SQLite database.

## Chosen approach

| Area | Choice | Reasoning |
| --- | --- | --- |
| HTTP | Gin | Gin gives the exercise concise, familiar REST routing while retaining explicit Go handlers. |
| Persistence | GORM with SQLite | GORM handles model mapping, transactions, and schema creation. Row models live in `internal/persistence`, so domain types carry no GORM tags; services still build their queries with GORM directly. |
| Migration | GORM `AutoMigrate` | The small, controlled schema is created reproducibly at startup; more complex production schema evolution would use reviewed versioned migrations. |
| SQLite dialect | `github.com/glebarez/sqlite` | This GORM dialect uses a pure-Go SQLite implementation, retaining the `CGO_ENABLED=0` Docker build. |
| Booking collision protection | `booking_nights` with `UNIQUE(room_id, stay_date)` | SQLite has no date-range exclusion constraint. Storing one row per occupied night lets a plain unique index reject any overlap. |
| Allocation | Pick the free room with the smallest adequate capacity, then the lowest room number | Allocation is deterministic and keeps scarce high-capacity rooms for larger parties. |
| Confirmation | Small notifier interface with a managed goroutine | It starts only after commit, can be faked in tests, and is waited for or cancelled at shutdown. |
| Operations | JSON `slog` for application logs, `/healthz`, SIGINT/SIGTERM shutdown | Modest production hygiene without extra platform infrastructure. Gin and GORM still write their own plain-text logs. |

## Components

```text
client
  │ HTTP/JSON
  ▼
Gin handlers ──► application services ──► GORM ──► SQLite file
                         │
                         └──────────────► notifier ──► structured log (after 2s)
```

- `cmd/api`: wires configuration, database, server, lifecycle, and logging.
- `internal/config`: parses environment variables without a framework.
- `internal/database`: opens/configures SQLite and runs GORM `AutoMigrate`.
- `internal/persistence`: holds GORM row models and converts them at the domain boundary.
- `internal/httpapi`: owns transport validation, HTTP status mapping, and response DTOs (hotel search returns the domain type directly).
- `internal/hotels`: finds named hotels and calculates suitable availability.
- `internal/bookings`: validates requests, selects a free room and writes the booking and its night rows in one transaction, and looks up references.
- `internal/notifications`: abstracts asynchronous confirmation delivery.
- `internal/evaluatordata`: seeds the evaluator inventory with fixed IDs and resets all data.

## Data and concurrency model

`hotels` owns `rooms`. Each booking references one room and stores the public reference, dates, party size, and lead guest. `booking_nights` holds one row per night, from check-in up to the night before checkout. The booking and all its night rows are written in one transaction.

Two layers stop a room being double-booked:

1. **Serialized writes.** The pool holds one SQLite connection, so booking transactions run one at a time. A second request for the last room sees it as occupied and gets `409`.
2. **The unique index.** `UNIQUE(room_id, stay_date)` rejects any overlap that reaches the database. The service maps that violation to the same `409`. With one connection this is a backstop; it becomes the primary protection once writes run concurrently.

Booking references use 12 cryptographically random base32 characters and are protected by a unique constraint. The confirmation notification starts only after the transaction commits. It waits two seconds and then writes a structured log line. On shutdown, the HTTP server drain and pending notifications share one five-second budget; notifications still pending after that are cancelled.

SQLite keeps setup cost near zero, and a single process with one connection is enough for this exercise. It is not a horizontally scaled, multi-writer design. Moving to PostgreSQL would keep the per-night unique constraint but would need query and error-handling changes in the services. Examples are the room-number `CAST` ordering, which relies on SQLite tolerating non-numeric text, the SQLite-specific duplicate-key detection, and retrying a reference collision inside a transaction, which Postgres would abort.

## Configuration and safety

| Variable | Default | Meaning |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listening port |
| `DATABASE_PATH` | `./data/hotel-bookings.db` | SQLite data file |
| `ENABLE_TEST_ENDPOINTS` | `true` | Enables unauthenticated seed/reset routes; set `false` outside local evaluation |

The API has no authentication because the brief does not require it. That makes the seed/reset endpoints dangerous on a public deployment, so they can be switched off and are documented as local-evaluation tools only.

## Verification and delivery

The Makefile provides reproducible local gates: `make fmt-check`, `make vet`, `make test`, and `make check`. Docker uses a multi-stage build and a non-root runtime user; Compose persists the SQLite database in a named volume. No CI workflow is included: for a small coding exercise, the Makefile targets are the review gate.

## Author completion

- **Architecture narrative in your own words:**
- **Alternatives considered and rejected:**
- **Most important trade-off:**
- **What you would change for multi-property or multi-instance production use:**
- **Review/demo talking points:**
