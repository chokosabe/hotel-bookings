FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/hotel-bookings ./cmd/api

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app && mkdir /data && chown app:app /data
COPY --from=build /out/hotel-bookings /usr/local/bin/hotel-bookings
USER app
ENV PORT=8080 DATABASE_PATH=/data/hotel-bookings.db ENABLE_TEST_ENDPOINTS=true
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/hotel-bookings"]
