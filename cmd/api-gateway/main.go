package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/atlaspay/platform/internal/auth"
	commonauth "github.com/atlaspay/platform/internal/common/auth"
	"github.com/atlaspay/platform/internal/common/cache"
	"github.com/atlaspay/platform/internal/common/config"
	"github.com/atlaspay/platform/internal/common/database"
	"github.com/atlaspay/platform/internal/common/dlq"
	"github.com/atlaspay/platform/internal/common/health"
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

	if err := database.ApplyMigrations(ctx, db.Pool, "migrations"); err != nil {
		logger.Fatal(ctx).Err(err).Msg("failed to apply database migrations")
	}
	logger.Info(ctx).Msg("database migrations applied")

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

	// Initialize the gateway-owned repository and remote bounded-context ports.
	authRepo := auth.NewRepository(db.Pool)
	dlqRepo := dlq.NewRepository(db.Pool)

	// Initialize services
	authService := auth.NewService(authRepo, jwtManager, cfg.JWT.RefreshExpiry)
	var paymentPort payment.Port
	if paymentURL := os.Getenv("PAYMENT_SERVICE_URL"); paymentURL != "" {
		paymentPort = payment.NewClient(paymentURL, os.Getenv("PAYMENT_SERVICE_TOKEN"))
		logger.Info(ctx).Str("url", paymentURL).Msg("using standalone payment service")
	} else {
		paymentPort = payment.NewService(payment.NewRepository(db.Pool))
	}
	var inventoryPort inventory.Port
	if inventoryURL := os.Getenv("INVENTORY_SERVICE_URL"); inventoryURL != "" {
		inventoryPort = inventory.NewClient(inventoryURL, os.Getenv("INVENTORY_SERVICE_TOKEN"))
		logger.Info(ctx).Str("url", inventoryURL).Msg("using standalone inventory service")
	} else {
		inventoryPort = inventory.NewService(inventory.NewRepository(db.Pool, redisCache))
	}

	var orderPort order.Port
	var orderConsumers []*kafka.Consumer
	if orderURL := os.Getenv("ORDER_SERVICE_URL"); orderURL != "" {
		orderPort = order.NewClient(orderURL, os.Getenv("ORDER_SERVICE_TOKEN"))
		logger.Info(ctx).Str("url", orderURL).Msg("using standalone order service")
	} else {
		var kafkaProducer *kafka.Producer
		dlqRepo := dlq.NewRepository(db.Pool)
		if cfg.Kafka.Enabled {
			kafkaProducer = kafka.NewProducer(cfg.Kafka.Brokers)
			defer kafkaProducer.Close()
		}
		orderService := order.NewServiceWithKafka(order.NewRepository(db.Pool, redisCache), inventoryPort, paymentPort, kafkaProducer, cfg.Kafka.Enabled)
		orderPort = orderService
		if cfg.Kafka.Enabled {
			workerCount := cfg.Kafka.Workers
			if workerCount < 1 {
				workerCount = 1
			}
			for i := 0; i < workerCount; i++ {
				orderConsumer := kafka.NewConsumerWithOptions(cfg.Kafka.Brokers, "atlaspay.orders", cfg.Kafka.GroupID+"-orders", orderService, dlqRepo, kafkaProducer)
				orderConsumers = append(orderConsumers, orderConsumer)
				go orderConsumer.Start(ctx)
			}
			orderService.StartOutboxPublisher(ctx)
			logger.Info(ctx).Str("topic", "atlaspay.orders").Int("workers", workerCount).Msg("Kafka order workers started")
		}
	}
	defer func() {
		for _, consumer := range orderConsumers {
			_ = consumer.Close()
		}
	}()

	// Initialize handlers
	authHandler := auth.NewHandler(authService)
	orderHandler := order.NewHandler(orderPort)
	paymentHandler := payment.NewHandler(paymentPort)
	inventoryHandler := inventory.NewHandler(inventoryPort)

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
	r.Get("/health/live", health.LiveHandler().ServeHTTP)
	r.Get("/health/ready", health.ReadyHandler(db.Health, redisHealth(redisCache)).ServeHTTP)
	r.Get("/ready", readinessCheck(db, redisCache))
	r.Handle("/metrics", metrics.Handler())

	// API routes (must come before static file server catch-all)
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

	// Serve static web UI (catch-all, must come last)
	fs := http.FileServer(http.Dir("./web"))
	r.Handle("/*", fs)

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

	// Start metrics polling goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := db.Stats()
				metrics.RecordDBConnections(int(stats.AcquiredConns()))
			}
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
				health["status"] = "degraded"
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

func readinessCheck(db *database.PostgresDB, redisCache *cache.RedisCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"ready":false}`))
			return
		}
		if err := redisHealth(redisCache)(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"ready":false}`))
			return
		}
		w.Write([]byte(`{"ready":true}`))
	}
}

func redisHealth(redisCache *cache.RedisCache) health.Check {
	return func(ctx context.Context) error {
		if redisCache == nil {
			return nil
		}
		return redisCache.Health(ctx)
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
