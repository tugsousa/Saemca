package config

import (
	"fmt"
	"log/slog"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
// The app will refuse to start if any required field is missing.
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Storage  StorageConfig
	Stripe   StripeConfig
	Email    EmailConfig
	SMS      SMSConfig
	Sentry   SentryConfig
}

type AppConfig struct {
	Env     string `env:"APP_ENV"      envDefault:"development"`
	Port    string `env:"APP_PORT"     envDefault:"8080"`
	BaseURL string `env:"APP_BASE_URL" envDefault:"http://localhost:8080"`
}

type DatabaseConfig struct {
	Host         string `env:"DB_HOST"           envDefault:"localhost"`
	Port         string `env:"DB_PORT"           envDefault:"5432"`
	User         string `env:"DB_USER"           required:"true"`
	Password     string `env:"DB_PASSWORD"       required:"true"`
	Name         string `env:"DB_NAME"           required:"true"`
	MaxOpenConns int    `env:"DB_MAX_OPEN_CONNS" envDefault:"10"`
}

// DSN builds the PostgreSQL connection string from individual fields.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		d.Host, d.Port, d.User, d.Password, d.Name,
	)
}

type AuthConfig struct {
	JWTSecret              string `env:"JWT_SECRET"                required:"true"`
	JWTExpiryMinutes       int    `env:"JWT_EXPIRY_MINUTES"        envDefault:"15"`
	RefreshTokenExpiryDays int    `env:"REFRESH_TOKEN_EXPIRY_DAYS" envDefault:"30"`
	CookieDomain           string `env:"COOKIE_DOMAIN"             envDefault:"localhost"`
	CookieSecure           bool   `env:"COOKIE_SECURE"             envDefault:"false"`
}

type StorageConfig struct {
	Endpoint  string `env:"STORAGE_ENDPOINT"   required:"true"`
	Region    string `env:"STORAGE_REGION"     envDefault:"auto"`
	Bucket    string `env:"STORAGE_BUCKET"     required:"true"`
	AccessKey string `env:"STORAGE_ACCESS_KEY" required:"true"`
	SecretKey string `env:"STORAGE_SECRET_KEY" required:"true"`
}

type StripeConfig struct {
	SecretKey          string `env:"STRIPE_SECRET_KEY"           required:"true"`
	WebhookSecret      string `env:"STRIPE_WEBHOOK_SECRET"       required:"true"`
	PlatformFeePercent int    `env:"STRIPE_PLATFORM_FEE_PERCENT" envDefault:"15"`
}

type EmailConfig struct {
	ResendAPIKey string `env:"RESEND_API_KEY" required:"true"`
	FromAddress  string `env:"EMAIL_FROM"     required:"true"`
}

type SMSConfig struct {
	VonageAPIKey    string `env:"VONAGE_API_KEY"    required:"true"`
	VonageAPISecret string `env:"VONAGE_API_SECRET" required:"true"`
	FromName        string `env:"VONAGE_FROM"       envDefault:"Salusdomi"`
}

type SentryConfig struct {
	DSN              string  `env:"SENTRY_DSN"`
	TracesSampleRate float64 `env:"SENTRY_TRACES_SAMPLE_RATE" envDefault:"0.1"`
}

// Load reads environment variables and returns a validated Config.
// In non-production environments it tries to load a .env file from
// several candidate paths (useful for running locally without Docker).
func Load() (*Config, error) {
	// Try loading a .env file for local development.
	// Silent failures are intentional — in production env vars are injected directly.
	for _, path := range []string{
		".env",
		"../infra/.env",
		"../../infra/.env",
	} {
		if err := godotenv.Load(path); err == nil {
			break
		}
	}

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// IsDevelopment returns true when running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsProduction returns true when running in production mode.
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

// LogValue implements slog.LogValuer.
// Logs a safe summary of config — never exposes secret values.
func (c *Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("env", c.App.Env),
		slog.String("port", c.App.Port),
		slog.String("db_host", c.Database.Host),
		slog.String("db_name", c.Database.Name),
		slog.Bool("sentry_enabled", c.Sentry.DSN != ""),
	)
}
