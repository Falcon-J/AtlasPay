package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atlaspay/platform/internal/common/cache"
	"github.com/atlaspay/platform/internal/common/config"
	"github.com/atlaspay/platform/internal/common/database"
	"github.com/atlaspay/platform/internal/common/logger"
	"github.com/atlaspay/platform/internal/common/response"
	"github.com/atlaspay/platform/internal/inventory"
	"github.com/go-chi/chi/v5"
)

func main() {
	logger.Init("inventory-service")
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	db, err := connectWithRetry(ctx, cfg.Database.DatabaseURL())
	if err != nil {
		logger.Fatal(ctx).Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	if migration, err := os.ReadFile("migrations/001_init.sql"); err == nil {
		if err := db.ExecScript(ctx, string(migration)); err != nil {
			logger.Fatal(ctx).Err(err).Msg("failed to run inventory migrations")
		}
	} else {
		logger.Fatal(ctx).Err(err).Msg("failed to read inventory migrations")
	}

	redisCache, err := cache.NewRedisCache(cfg.Redis.RedisAddr(), cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		logger.Fatal(ctx).Err(err).Msg("failed to connect to Redis")
	}
	defer redisCache.Close()

	service := inventory.NewService(inventory.NewRepository(db.Pool, redisCache))
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Health(r.Context()); err != nil {
			response.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "dependency": "postgres"})
			return
		}
		if err := redisCache.Health(r.Context()); err != nil {
			response.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "dependency": "redis"})
			return
		}
		response.OK(w, map[string]string{"status": "healthy"})
	})
	r.Mount("/", inventory.NewInternalHandler(service, os.Getenv("INVENTORY_SERVICE_TOKEN")))

	port := os.Getenv("INVENTORY_SERVICE_PORT")
	if port == "" {
		port = "8082"
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info(ctx).Str("port", port).Msg("inventory service started")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal(ctx).Err(err).Msg("inventory service failed")
	}
}

func connectWithRetry(ctx context.Context, databaseURL string) (*database.PostgresDB, error) {
	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		db, err := database.NewPostgresDB(ctx, databaseURL)
		if err == nil {
			return db, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
	return nil, lastErr
}
