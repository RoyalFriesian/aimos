package postgres

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const defaultEnvFile = ".env"

type Config struct {
	DatabaseURL string
	Host        string
	Port        int
	DBName      string
	User        string
	Password    string
	SSLMode     string
	MaxConns    int32
	MinConns    int32
}

func LoadConfig(envFile string) (Config, error) {
	if envFile == "" {
		envFile = defaultEnvFile
	}
	if err := godotenv.Load(envFile); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("load env file: %w", err)
	}

	port, err := intFromEnv("POSTGRES_PORT", 5432)
	if err != nil {
		return Config{}, err
	}
	maxConns, err := int32FromEnv("POSTGRES_MAX_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	minConns, err := int32FromEnv("POSTGRES_MIN_CONNS", 2)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Host:        getenvDefault("POSTGRES_HOST", "localhost"),
		Port:        port,
		DBName:      getenvDefault("POSTGRES_DB", "sarnga"),
		User:        getenvDefault("POSTGRES_USER", "sarnga"),
		Password:    getenvDefault("POSTGRES_PASSWORD", "sarnga_dev_password"),
		SSLMode:     getenvDefault("POSTGRES_SSLMODE", "disable"),
		MaxConns:    maxConns,
		MinConns:    minConns,
	}

	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) Validate() error {
	if c.DatabaseURL != "" {
		return nil
	}
	if c.Host == "" {
		return fmt.Errorf("POSTGRES_HOST is required")
	}
	if c.Port <= 0 {
		return fmt.Errorf("POSTGRES_PORT must be greater than 0")
	}
	if c.DBName == "" {
		return fmt.Errorf("POSTGRES_DB is required")
	}
	if c.User == "" {
		return fmt.Errorf("POSTGRES_USER is required")
	}
	if c.SSLMode == "" {
		return fmt.Errorf("POSTGRES_SSLMODE is required")
	}
	return nil
}

func (c Config) ConnectionString() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		url.QueryEscape(c.User),
		url.QueryEscape(c.Password),
		c.Host,
		c.Port,
		url.PathEscape(c.DBName),
		url.QueryEscape(c.SSLMode),
	)
}

func OpenPool(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(config.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	poolConfig.MaxConns = config.MaxConns
	poolConfig.MinConns = config.MinConns
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}

func getenvDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func intFromEnv(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return parsed, nil
}

func int32FromEnv(key string, fallback int32) (int32, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return int32(parsed), nil
}
