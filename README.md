# Hotel Bookings API

A Go/Gin REST API for the hotel-booking coding exercise. It is intentionally small, but is designed around the non-negotiable booking rules: a party receives one suitable room for its entire stay and a room cannot be double-booked for a night.

> **Status:** the foundation slice is runnable. Hotel search, availability, booking, retrieval, and seed/reset endpoints will be added as separately committed slices.

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

## Documentation

- [Product requirements and assumptions](docs/PRD.md)
- [Architecture and trade-offs](docs/ARCHITECTURE.md)
- [OpenAPI contract](docs/openapi.yaml)
- [Executable HTTP requests](requests.http)
- [Implementation plan](docs/superpowers/plans/2026-10-01-hotel-booking-api.md)

## AI assistance disclosure

AI assistance was used to accelerate scaffolding, tests, and documentation. The repository author is responsible for reviewing the requirements, architectural choices, implementation, test results, and final submission.
