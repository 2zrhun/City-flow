package config

import (
	"os"
	"strings"
	"testing"
)

func TestGetDSN(t *testing.T) {
	db := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "cityflow",
		Password: "secret",
		Name:     "cityflow",
		SSLMode:  "disable",
	}
	dsn := db.GetDSN()

	expected := "host=localhost port=5432 user=cityflow password=secret dbname=cityflow sslmode=disable"
	if dsn != expected {
		t.Errorf("GetDSN() = %q, want %q", dsn, expected)
	}
}

func TestGetDSNCustomValues(t *testing.T) {
	db := DatabaseConfig{
		Host:     "db.example.com",
		Port:     5433,
		User:     "admin",
		Password: "p@ss",
		Name:     "mydb",
		SSLMode:  "require",
	}
	dsn := db.GetDSN()

	if !strings.Contains(dsn, "host=db.example.com") {
		t.Errorf("DSN missing host, got: %s", dsn)
	}
	if !strings.Contains(dsn, "port=5433") {
		t.Errorf("DSN missing port, got: %s", dsn)
	}
	if !strings.Contains(dsn, "sslmode=require") {
		t.Errorf("DSN missing sslmode, got: %s", dsn)
	}
}

func TestGetEnv(t *testing.T) {
	os.Unsetenv("TEST_CONFIG_VAR")
	if got := getEnv("TEST_CONFIG_VAR", "default"); got != "default" {
		t.Errorf("getEnv() = %q, want %q", got, "default")
	}

	os.Setenv("TEST_CONFIG_VAR", "custom")
	defer os.Unsetenv("TEST_CONFIG_VAR")
	if got := getEnv("TEST_CONFIG_VAR", "default"); got != "custom" {
		t.Errorf("getEnv() = %q, want %q", got, "custom")
	}
}

func TestGetIntEnv(t *testing.T) {
	t.Run("fallback when unset", func(t *testing.T) {
		os.Unsetenv("TEST_INT_VAR")
		got, err := getIntEnv("TEST_INT_VAR", 8080)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 8080 {
			t.Errorf("getIntEnv() = %d, want %d", got, 8080)
		}
	})

	t.Run("parses valid int", func(t *testing.T) {
		os.Setenv("TEST_INT_VAR", "9090")
		defer os.Unsetenv("TEST_INT_VAR")
		got, err := getIntEnv("TEST_INT_VAR", 8080)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 9090 {
			t.Errorf("getIntEnv() = %d, want %d", got, 9090)
		}
	})

	t.Run("error on invalid int", func(t *testing.T) {
		os.Setenv("TEST_INT_VAR", "not_int")
		defer os.Unsetenv("TEST_INT_VAR")
		_, err := getIntEnv("TEST_INT_VAR", 8080)
		if err == nil {
			t.Error("expected error for invalid int value")
		}
	})
}

func TestLoadConfigDefaults(t *testing.T) {
	// Clear env vars to get defaults
	for _, key := range []string{"SERVER_PORT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE", "JWT_SECRET", "JWT_EXPIRY_HOURS", "REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD", "REDIS_DB", "CORS_ALLOWED_ORIGINS", "WS_POLL_INTERVAL_MS"} {
		os.Unsetenv(key)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "localhost")
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Database.Port = %d, want 5432", cfg.Database.Port)
	}
	if cfg.JWT.ExpiryHours != 24 {
		t.Errorf("JWT.ExpiryHours = %d, want 24", cfg.JWT.ExpiryHours)
	}
	if cfg.Redis.Port != 6379 {
		t.Errorf("Redis.Port = %d, want 6379", cfg.Redis.Port)
	}
	if cfg.CORS.AllowedOrigins != "*" {
		t.Errorf("CORS.AllowedOrigins = %q, want %q", cfg.CORS.AllowedOrigins, "*")
	}
}

func TestLoadConfigCustom(t *testing.T) {
	os.Setenv("SERVER_PORT", "3000")
	os.Setenv("DB_HOST", "db.prod")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("JWT_EXPIRY_HOURS", "48")
	defer func() {
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("JWT_EXPIRY_HOURS")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.Server.Port != 3000 {
		t.Errorf("Server.Port = %d, want 3000", cfg.Server.Port)
	}
	if cfg.Database.Host != "db.prod" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "db.prod")
	}
	if cfg.Database.Port != 5433 {
		t.Errorf("Database.Port = %d, want 5433", cfg.Database.Port)
	}
	if cfg.JWT.ExpiryHours != 48 {
		t.Errorf("JWT.ExpiryHours = %d, want 48", cfg.JWT.ExpiryHours)
	}
}

func TestLoadConfigInvalidPort(t *testing.T) {
	os.Setenv("SERVER_PORT", "invalid")
	defer os.Unsetenv("SERVER_PORT")

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid SERVER_PORT")
	}
}
