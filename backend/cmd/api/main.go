package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"salusdomi.com/api/internal/auth"
	"salusdomi.com/api/internal/config"
	"salusdomi.com/api/platform"
)

func main() {
	// ── 1. Config ────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		// Use plain log here — slog isn't set up yet.
		fmt.Fprintf(os.Stderr, "FATAL: failed to load config: %v\n", err)
		os.Exit(1)
	}

	// ── 2. Logger ────────────────────────────────────────────────────────────
	logger := newLogger(cfg)
	slog.SetDefault(logger)
	slog.Info("config loaded", "config", cfg)

	// ── 3. Sentry ────────────────────────────────────────────────────────────
	if cfg.Sentry.DSN != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              cfg.Sentry.DSN,
			Environment:      cfg.App.Env,
			TracesSampleRate: cfg.Sentry.TracesSampleRate,
		}); err != nil {
			slog.Warn("sentry init failed — continuing without error tracking", "error", err)
		} else {
			slog.Info("sentry initialized")
			defer sentry.Flush(2 * time.Second)
		}
	}

	// ── 4. Database ───────────────────────────────────────────────────────────
	db, err := platform.NewDB(cfg.Database)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("database connected", "host", cfg.Database.Host, "name", cfg.Database.Name)

	// ── 5. Router ─────────────────────────────────────────────────────────────
	r := buildRouter(cfg, db)

	// ── 6. HTTP Server with graceful shutdown ─────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine so it doesn't block the shutdown listener.
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.App.Port, "env", cfg.App.Env)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Wait for interrupt signal (Ctrl+C) or a server error.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		slog.Error("server error", "error", err)
	case sig := <-quit:
		slog.Info("shutdown signal received", "signal", sig)
	}

	// Give active requests 10 seconds to finish.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped cleanly")
}

// buildRouter wires up all middleware and routes.
func buildRouter(cfg *config.Config, db *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()

	// ── Global middleware ────────────────────────────────────────────────────
	r.Use(middleware.RequestID)  // Adds X-Request-ID header (used as trace_id)
	r.Use(middleware.RealIP)     // Reads X-Forwarded-For / X-Real-IP from Nginx
	r.Use(middleware.Recoverer)  // Recovers from panics and returns 500

	// Structured request logging
	r.Use(requestLogger)

	// CORS — allow the frontend origin
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.App.BaseURL, "http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true, // Required for httpOnly cookie (refresh token)
		MaxAge:           300,
	}))

	// Sentry HTTP middleware — captures panics and request context
	if cfg.Sentry.DSN != "" {
		sentryMiddleware := sentryhttp.New(sentryhttp.Options{Repanic: true})
		r.Use(sentryMiddleware.Handle)
	}

	// ── Auth domain ───────────────────────────────────────────────────────────
	authRepo := auth.NewRepository(db)
	authSvc := auth.NewService(authRepo, cfg.Auth)
	authHandler := auth.NewHandler(authSvc, cfg.Auth)

	// ── Routes ───────────────────────────────────────────────────────────────
	r.Get("/health", healthHandler(db))
	r.Mount("/auth", authHandler.Routes())

	return r
}

// healthHandler returns the API and DB status.
// Used by Docker, load balancers, and uptime monitors.
func healthHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			slog.ErrorContext(r.Context(), "health check db ping failed", "error", err)
			dbStatus = "unreachable"
		}

		status := http.StatusOK
		if dbStatus != "ok" {
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"status":"ok","db":"%s"}`, dbStatus)
	}
}

// requestLogger is a middleware that logs each request using slog.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		next.ServeHTTP(ww, r)

		slog.InfoContext(r.Context(), "request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.Status()),
			slog.Duration("duration", time.Since(start)),
			slog.String("trace_id", middleware.GetReqID(r.Context())),
		)
	})
}

// newLogger creates a slog logger: JSON in production, human-readable in development.
func newLogger(cfg *config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
