package config

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	// Test existing env
	os.Setenv("TEST_ENV_VAR", "hello_world")
	defer os.Unsetenv("TEST_ENV_VAR")

	val := getEnv("TEST_ENV_VAR", "fallback")
	if val != "hello_world" {
		t.Errorf("Expected 'hello_world', got '%s'", val)
	}

	// Test fallback
	valFallback := getEnv("NON_EXISTENT_VAR_123", "fallback_val")
	if valFallback != "fallback_val" {
		t.Errorf("Expected 'fallback_val', got '%s'", valFallback)
	}
}

func TestLoadConfig(t *testing.T) {
	cfg := LoadConfig()
	if cfg == nil {
		t.Fatal("Expected non-nil Config")
	}

	if cfg.AppPort == "" {
		t.Error("Expected default AppPort to be non-empty")
	}

	if cfg.JWTSecret == "" {
		t.Error("Expected JWTSecret to be non-empty")
	}

	if cfg.WorkerSecret == "" {
		t.Error("Expected WorkerSecret to be non-empty")
	}
}
