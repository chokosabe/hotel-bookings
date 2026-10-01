# Hotel Bookings API — Architecture

## Overview

The API is a single Go process. Gin is responsible only for HTTP routing and response encoding; application services own hotel lookup, availability, test-data lifecycle, and booking decisions; `database/sql` owns persistence. This keeps HTTP concerns out of the booking rules and makes the services testable with a real temporary SQLite database.

## Chosen approach

| Area | Choice | Reasoning |
| --- | --- | --- |
| HTTP | Gin | Gin gives the exercise concise, familiar REST routing while retaining explicit Go handlers. |
| Persistence | `database/sql` with SQLite | A single file gives reviewers a zero-dependency local setup and avoids ORM-generated behaviour. |
| Migration | Embedded versioned SQL | Schema creation is reproducible at startup and visible in the repository. |
| Booking collision protection | `booking_nights` with `UNIQUE(room_id, stay_date)` | SQLite cannot express a native date-range exclusion constraint. Materialising each occupied night turns overlap protection into a database-enforced invariant and works safely in a transaction. |
| Allocation | Query suitable unoccupied rooms by capacity then room number | Allocation is deterministic and preserves scarce high-capacity rooms. |
| Confirmation | Small notifier interface with managed goroutine | It starts only after commit, can be faked in tests, and avoids abandoned goroutines during shutdown. |
| Operations | JSON `slog`, `/healthz`, SIGINT/SIGTERM shutdown | This is modest production hygiene without unnecessary platform infrastructure. |

## Components

```text
client
  │ HTTP/JSON
  ▼
Gin handlers ──► application services ──► database/sql ──► SQLite file
                         │
                         └──────────────► notifier ──► structured log (after 2s)
```

- `cmd/api`: wires configuration, database, server, lifecycle, and logging.
- `internal/config`: parses environment variables without a framework.
- `internal/database`: opens SQLite, enables foreign keys, and runs embedded migrations.
- `internal/httpapi`: owns only transport validation, HTTP status mapping, and DTOs.
- `internal/hotels`: finds named hotels and calculates suitable availability.
- `internal/bookings`: validates requests, selects/locks a room through the database transaction, creates booking-night rows, and looks up references.
- `internal/notifications`: abstracts asynchronous confirmation delivery.
- `internal/evaluatordata`: creates and removes deterministic evaluator data.

## Data and concurrency model

`hotels` owns `rooms`; `bookings` references one room and captures the public reference, dates, party size, and lead guest. `booking_nights` contains one row per night from check-in through the day before checkout. Creating the booking and every night row is one transaction. A unique conflict on `(room_id, stay_date)` is translated to HTTP `409`, so two simultaneous requests cannot both reserve the final room.

SQLite is deliberately selected for low setup cost. This deployment is a single application process using one configured SQLite connection; it is appropriate for the challenge rather than a horizontally scaled, multi-writer production deployment. A future multi-instance design would migrate to PostgreSQL while retaining the service interfaces and use equivalent transactional constraints.

## Configuration and safety

| Variable | Default | Meaning |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listening port |
| `DATABASE_PATH` | `./data/hotel-bookings.db` | SQLite data file |
| `ENABLE_TEST_ENDPOINTS` | `true` | Enables unauthenticated seed/reset routes; set `false` outside local evaluation |

The API has no authentication because the brief explicitly requires none. This makes test endpoints dangerous in a public deployment; they are therefore configurable and documented as local-evaluation tools only.

## Verification and delivery

The Makefile provides reproducible local gates: `make fmt-check`, `make vet`, `make test`, and `make check`. Docker uses a multi-stage build and a non-root runtime user; Compose persists the SQLite database in a named volume. No CI workflow is included by design: the local Makefile is the requested review gate for a small coding exercise.

## Author completion

- **Architecture narrative in your own words:**
- **Alternatives considered and rejected:**
- **Most important trade-off:**
- **What you would change for multi-property or multi-instance production use:**
- **Review/demo talking points:**
