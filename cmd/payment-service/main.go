package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atlaspay/platform/internal/common/config"
	"github.com/atlaspay/platform/internal/common/database"
	"github.com/atlaspay/platform/internal/common/logger"
	"github.com/atlaspay/platform/internal/common/response"
	"github.com/atlaspay/platform/internal/payment"
	"github.com/go-chi/chi/v5"
)

func main() {
	logger.Init("payment-service")
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
			logger.Fatal(ctx).Err(err).Msg("failed to run payment migrations")
		}
	} else {
		logger.Fatal(ctx).Err(err).Msg("failed to read payment migrations")
	}

	service := payment.NewService(payment.NewRepository(db.Pool))
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Health(r.Context()); err != nil {
			response.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
			return
		}
		response.OK(w, map[string]string{"status": "healthy"})
	})
	r.Mount("/", payment.NewInternalHandler(service, os.Getenv("PAYMENT_SERVICE_TOKEN")))

	port := os.Getenv("PAYMENT_SERVICE_PORT")
	if port == "" {
		port = "8081"
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

	logger.Info(ctx).Str("port", port).Msg("payment service started")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal(ctx).Err(err).Msg("payment service failed")
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
