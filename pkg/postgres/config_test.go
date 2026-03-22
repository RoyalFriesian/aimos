package postgres

import (
	"strings"
	"testing"
)

func TestConnectionStringUsesDatabaseURLWhenPresent(t *testing.T) {
	config := Config{DatabaseURL: "postgres://example"}
	if got := config.ConnectionString(); got != "postgres://example" {
		t.Fatalf("expected database url passthrough, got %q", got)
	}
}

func TestConnectionStringBuildsFromParts(t *testing.T) {
	config := Config{
		Host:     "localhost",
		Port:     5432,
		DBName:   "sarnga",
		User:     "sarnga",
		Password: "secret",
		SSLMode:  "disable",
	}
	got := config.ConnectionString()
	if !strings.Contains(got, "postgres://sarnga:secret@localhost:5432/sarnga?sslmode=disable") {
		t.Fatalf("unexpected connection string: %q", got)
	}
}

func TestLoadConfigReadsDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_HOST", "")
	t.Setenv("POSTGRES_PORT", "")
	t.Setenv("POSTGRES_DB", "")
	t.Setenv("POSTGRES_USER", "")
	t.Setenv("POSTGRES_PASSWORD", "")
	t.Setenv("POSTGRES_SSLMODE", "")
	t.Setenv("POSTGRES_MAX_CONNS", "")
	t.Setenv("POSTGRES_MIN_CONNS", "")

	config, err := LoadConfig("missing.env")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if config.Host != "localhost" || config.DBName != "sarnga" || config.User != "sarnga" {
		t.Fatalf("unexpected defaults: %#v", config)
	}
	if config.Port != 5432 || config.MaxConns != 10 || config.MinConns != 2 {
		t.Fatalf("unexpected numeric defaults: %#v", config)
	}
}
