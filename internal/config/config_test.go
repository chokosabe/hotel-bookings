package config_test

import (
	"testing"

	"github.com/chokosabe/hotel-bookings/internal/config"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(func(string) string { return "" })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.DatabasePath != "./data/hotel-bookings.db" {
		t.Errorf("DatabasePath = %q, want default path", cfg.DatabasePath)
	}
	if !cfg.EnableTestEndpoints {
		t.Error("EnableTestEndpoints = false, want true")
	}
}

func TestLoadRejectsInvalidTestEndpointFlag(t *testing.T) {
	t.Parallel()

	_, err := config.Load(func(key string) string {
		if key == "ENABLE_TEST_ENDPOINTS" {
			return "sometimes"
		}
		return ""
	})
	if err == nil {
		t.Fatal("Load() error = nil, want invalid flag error")
	}
}
