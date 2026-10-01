# Hotel Bookings API

A Go/Gin REST API for the hotel-booking coding exercise. It is intentionally small, but is designed around the non-negotiable booking rules: a party receives one suitable room for its entire stay and a room cannot be double-booked for a night.

> **Status:** all challenge flows are implemented: search, availability, atomic booking creation with simulated confirmation, and reference lookup.

## Quick start

Prerequisites: Go 1.25+ and Docker/Compose (optional).

```bash
make run
curl http://localhost:8080/healthz
```

Or run the container:

```bash
docker compose up --build
curl http://localhost:8080/healthz
docker compose down
```

## Local checks

```bash
make fmt       # format Go files
make fmt-check # fail if formatting is needed
make vet
make test
make check     # fmt-check + vet + test
```

## Configuration

Copy `.env.example` if you want a record of local values; the API reads environment variables directly.

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listening port |
| `DATABASE_PATH` | `./data/hotel-bookings.db` | SQLite database file |
| `ENABLE_TEST_ENDPOINTS` | `true` | Enables unauthenticated evaluator seed/reset endpoints; set to `false` in deployments |

The service has no authentication because the challenge does not require it. Never expose enabled seed/reset endpoints on an untrusted public deployment.

## Current endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/healthz` | Process health check |
| `POST` | `/api/v1/test/reset` | Remove all seeded test data |
| `POST` | `/api/v1/test/seed` | Idempotently create The Grand Hotel and six rooms |
| `GET` | `/api/v1/hotels?name=grand` | Find hotels by case-insensitive name substring |
| `GET` | `/api/v1/hotels/{hotelID}/availability?...` | Find rooms free for every requested night and suitable for the party |
| `POST` | `/api/v1/bookings` | Atomically assign and reserve one room for a complete stay |
| `GET` | `/api/v1/bookings/{reference}` | Retrieve a booking and its assigned room |

The test endpoints return `404` when `ENABLE_TEST_ENDPOINTS=false`.

## End-to-end walkthrough

1. Start the API with `make run`.
2. Run reset and seed from [`requests.http`](requests.http).
3. Search `GET /api/v1/hotels?name=grand`; use the returned hotel ID.
4. Check availability with `check_in`, `check_out`, and `guests` query parameters.
5. Create a booking and retain its `reference` from the `201` response.
6. Retrieve the same booking from `GET /api/v1/bookings/{reference}`.

A successful creation reserves each night in one transaction, then logs the simulated confirmation after two seconds.

## Documentation

- [Product requirements and assumptions](docs/PRD.md)
- [Architecture and trade-offs](docs/ARCHITECTURE.md)
- [OpenAPI contract](docs/openapi.yaml)
- [Executable HTTP requests](requests.http)
- [Implementation plan](docs/superpowers/plans/2026-10-01-hotel-booking-api.md)

## AI assistance disclosure

AI assistance was used to accelerate scaffolding, tests, and documentation. The repository author is responsible for reviewing the requirements, architectural choices, implementation, test results, and final submission.
