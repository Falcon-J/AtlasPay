package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atlaspay/platform/internal/auth"
	commonauth "github.com/atlaspay/platform/internal/common/auth"
	"github.com/atlaspay/platform/internal/common/cache"
	"github.com/atlaspay/platform/internal/common/config"
	"github.com/atlaspay/platform/internal/common/database"
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
	ctx := context.Background()

	// Load configuration
	cfg := config.Load()

	logger.Info(ctx).Str("port", cfg.Server.Port).Msg("starting API Gateway")

	// Initialize database
	db, err := database.NewPostgresDB(ctx, cfg.Database.DatabaseURL())
	if err != nil {
		logger.Fatal(ctx).Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	// Run migrations (auto-initialize schema)
	migrationSQL, err := os.ReadFile("./migrations/001_init.sql")
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

	// Initialize services
	authService := auth.NewService(authRepo, jwtManager, cfg.JWT.RefreshExpiry)
	orderService := order.NewService(orderRepo)
	paymentService := payment.NewService(paymentRepo)
	inventoryService := inventory.NewService(inventoryRepo)

	// Initialize handlers
	authHandler := auth.NewHandler(authService)
	orderHandler := order.NewHandler(orderService)
	paymentHandler := payment.NewHandler(paymentService)
	inventoryHandler := inventory.NewHandler(inventoryService)

	// Initialize rate limiter (100 requests per minute, burst of 10)
	rateLimiter := middleware.NewRateLimiter(100, time.Minute, 10)

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
