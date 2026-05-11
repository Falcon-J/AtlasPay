package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/atlaspay/platform/internal/auth"
	commonauth "github.com/atlaspay/platform/internal/common/auth"
	"github.com/atlaspay/platform/internal/common/cache"
	"github.com/atlaspay/platform/internal/common/config"
	"github.com/atlaspay/platform/internal/common/database"
	"github.com/atlaspay/platform/internal/common/dlq"
	"github.com/atlaspay/platform/internal/common/kafka"
	"github.com/atlaspay/platform/internal/common/logger"
	"github.com/atlaspay/platform/internal/common/metrics"
	"github.com/atlaspay/platform/internal/common/middleware"
	"github.com/atlaspay/platform/internal/inventory"
	"github.com/atlaspay/platform/internal/order"
	"github.com/atlaspay/platform/internal/payment"
	"github.com/go-chi/chi/v5"
)

func main() {
	// Initialize logger
	logger.Init("api-gateway")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg := config.Load()

	logger.Info(ctx).Str("port", cfg.Server.Port).Msg("starting API Gateway")

	// Initialize database with retry (Render provisions DB async)
	db, err := connectWithRetry(ctx, cfg.Database.DatabaseURL())
	if err != nil {
		logger.Fatal(ctx).Err(err).Msg("failed to connect to database after retries")
	}
	defer db.Close()

	logger.Info(ctx).Msg("database connection established successfully")

	// Run migrations (auto-initialize schema)
	migrationSQL, err := readMigrationFile(ctx)
	if err != nil {
		logger.Warn(ctx).Err(err).Msg("failed to read migration file, skipping auto-migration")
	} else {
		if err := db.ExecScript(ctx, string(migrationSQL)); err != nil {
			logger.Error(ctx).Err(err).Msg("failed to run migrations")
		} else {
			logger.Info(ctx).Msg("auto-migration completed successfully")
		}
	}

	// Initialize Redis cache
	redisCache, err := cache.NewRedisCache(cfg.Redis.RedisAddr(), cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		logger.Warn(ctx).Err(err).Msg("failed to connect to Redis, continuing without cache")
		redisCache = nil
	} else {
		defer redisCache.Close()
	}

	// Initialize JWT manager
	jwtManager := commonauth.NewJWTManager(
		cfg.JWT.AccessSecret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessExpiry,
		cfg.JWT.RefreshExpiry,
	)

	// Initialize repositories
	authRepo := auth.NewRepository(db.Pool)
	orderRepo := order.NewRepository(db.Pool, redisCache)
	paymentRepo := payment.NewRepository(db.Pool)
	inventoryRepo := inventory.NewRepository(db.Pool, redisCache)
	dlqRepo := dlq.NewRepository(db.Pool)

	var kafkaProducer *kafka.Producer
	if cfg.Kafka.Enabled {
		kafkaProducer = kafka.NewProducer(cfg.Kafka.Brokers)
		defer kafkaProducer.Close()
	}

	// Initialize services
	authService := auth.NewService(authRepo, jwtManager, cfg.JWT.RefreshExpiry)
	paymentService := payment.NewService(paymentRepo)
	inventoryService := inventory.NewService(inventoryRepo)
	orderService := order.NewServiceWithKafka(orderRepo, inventoryService, paymentService, kafkaProducer, cfg.Kafka.Enabled)

	var orderConsumer *kafka.Consumer
	if cfg.Kafka.Enabled {
		orderConsumer = kafka.NewConsumerWithOptions(
			cfg.Kafka.Brokers,
			"atlaspay.orders",
			cfg.Kafka.GroupID+"-orders",
			orderService,
			dlqRepo,
			kafkaProducer,
		)
		defer orderConsumer.Close()
		go orderConsumer.Start(ctx)
		logger.Info(ctx).Str("topic", "atlaspay.orders").Msg("Kafka order worker started")
	}

	// Initialize handlers
	authHandler := auth.NewHandler(authService)
	orderHandler := order.NewHandler(orderService)
	paymentHandler := payment.NewHandler(paymentService)
	inventoryHandler := inventory.NewHandler(inventoryService)

	// Initialize rate limiter
	rateLimiter := middleware.NewRateLimiter(cfg.Server.RateLimit, time.Minute, cfg.Server.RateBurst)

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Recoverer)
	r.Use(middleware.CORS)
	r.Use(middleware.RequestLogger)
	r.Use(middleware.RateLimit(rateLimiter))
	r.Use(middleware.Timeout(30 * time.Second))

	// Health endpoints
	r.Get("/health", healthCheck(db, redisCache))
	r.Get("/ready", readinessCheck(db))
	r.Handle("/metrics", metrics.Handler())

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Public routes (no auth required)
		r.Mount("/auth", authHandler.Routes())

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(jwtManager))

			r.Mount("/orders", orderHandler.Routes())
			r.Mount("/payments", paymentHandler.Routes())
			r.Mount("/inventory", inventoryHandler.Routes())
		})

		// Admin routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(jwtManager))
			r.Use(middleware.RequireRole(commonauth.RoleAdmin))

			// Admin-only endpoints would go here
			r.Get("/admin/stats", adminStats())
			r.Get("/admin/dlq", adminDLQ(dlqRepo))
		})
	})

	// Create server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info(ctx).Str("port", cfg.Server.Port).Msg("API Gateway listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(ctx).Err(err).Msg("server failed")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx).Msg("shutting down server...")
	cancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx).Err(err).Msg("server forced to shutdown")
	}

	logger.Info(ctx).Msg("server stopped")
}

func healthCheck(db *database.PostgresDB, cache *cache.RedisCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := map[string]string{
			"status": "healthy",
			"db":     "up",
			"cache":  "up",
		}

		ctx := r.Context()

		// Check database
		if err := db.Health(ctx); err != nil {
			health["db"] = "down"
			health["status"] = "degraded"
		}

		// Check cache
		if cache != nil {
			if err := cache.Health(ctx); err != nil {
				health["cache"] = "down"
			}
		} else {
			health["cache"] = "not configured"
		}

		w.Header().Set("Content-Type", "application/json")
		if health["status"] != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		w.Write([]byte(`{"status":"` + health["status"] + `","db":"` + health["db"] + `","cache":"` + health["cache"] + `"}`))
	}
}

func readinessCheck(db *database.PostgresDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"ready":false}`))
			return
		}
		w.Write([]byte(`{"ready":true}`))
	}
}

func adminStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Admin stats endpoint placeholder
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"admin stats endpoint"}`))
	}
}

func adminDLQ(repo *dlq.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		events, err := repo.ListRecent(r.Context(), limit)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"failed to list dead-letter events"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"events": events})
	}
}

// connectWithRetry attempts to connect to PostgreSQL with exponential backoff
// Render provisions databases asynchronously (can take 60-120s), so we need aggressive retry
func connectWithRetry(ctx context.Context, dbURL string) (*database.PostgresDB, error) {
	maxAttempts := 20
	backoff := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		db, err := database.NewPostgresDB(ctx, dbURL)
		if err == nil {
			logger.Info(ctx).Int("attempt", attempt).Msg("database connected successfully")
			return db, nil
		}

		if attempt < maxAttempts {
			logger.Warn(ctx).
				Err(err).
				Int("attempt", attempt).
				Int("max_attempts", maxAttempts).
				Dur("retry_in", backoff).
				Msg("database connection failed, retrying...")

			select {
			case <-time.After(backoff):
				backoff = time.Duration(float64(backoff) * 1.5) // Exponential backoff: 2s, 3s, 4.5s, 6.75s...
				if backoff > 60*time.Second {
					backoff = 60 * time.Second // Cap at 60s for Render's slow provisioning
				}
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during database retry")
			}
		} else {
			return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxAttempts, err)
		}
	}

	return nil, fmt.Errorf("database connection exhausted all retry attempts")
}

// readMigrationFile attempts to read migration file from multiple possible paths
// This handles different deployment scenarios (local, docker, render)
func readMigrationFile(ctx context.Context) ([]byte, error) {
	possiblePaths := []string{
		"./migrations/001_init.sql",
		"migrations/001_init.sql",
		"/app/migrations/001_init.sql",
		"../scripts/migrations/001_init.sql",
		"scripts/migrations/001_init.sql",
	}

	// Also try based on executable directory
	if ex, err := os.Executable(); err == nil {
		exePath := filepath.Dir(ex)
		possiblePaths = append(possiblePaths,
			filepath.Join(exePath, "migrations/001_init.sql"),
			filepath.Join(exePath, "../scripts/migrations/001_init.sql"),
		)
	}

	var lastErr error
	for _, path := range possiblePaths {
		data, err := os.ReadFile(path)
		if err == nil {
			logger.Info(ctx).Str("path", path).Msg("migration file loaded")
			return data, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("migration file not found in any of %d paths (last error: %w)", len(possiblePaths), lastErr)
}
