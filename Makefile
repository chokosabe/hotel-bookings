.PHONY: fmt fmt-check vet test run build docker-up docker-down check

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || (echo 'Go files are not formatted. Run make fmt.' && exit 1)

vet:
	go vet ./...

test:
	go test ./...

run:
	go run ./cmd/api

build:
	go build -o bin/hotel-bookings ./cmd/api

docker-up:
	docker compose up --build

docker-down:
	docker compose down

check: fmt-check vet test
