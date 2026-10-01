package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chokosabe/hotel-bookings/internal/bookings"
	"github.com/chokosabe/hotel-bookings/internal/config"
	"github.com/chokosabe/hotel-bookings/internal/database"
	"github.com/chokosabe/hotel-bookings/internal/evaluatordata"
	"github.com/chokosabe/hotel-bookings/internal/hotels"
	"github.com/chokosabe/hotel-bookings/internal/httpapi"
	"github.com/chokosabe/hotel-bookings/internal/notifications"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabasePath)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("access database pool", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	notifier := notifications.NewAsyncNotifier(logger, 2*time.Second)
	defer notifier.Close()

	server := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: httpapi.NewHandler(httpapi.Dependencies{
			Bookings:            bookings.NewService(db, notifier),
			Hotels:              hotels.NewService(db),
			TestData:            evaluatordata.NewService(db),
			EnableTestEndpoints: cfg.EnableTestEndpoints,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	logger.Info("server started", "port", cfg.Port, "database_path", cfg.DatabasePath)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown requested")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
		if err := notifier.Wait(shutdownCtx); err != nil {
			logger.Warn("confirmation notifications did not finish before shutdown", "error", err)
			notifier.Close()
		}
	}
}
