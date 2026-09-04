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
	"github.com/atlaspay/platform/internal/common/health"
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
	db, err := database.ConnectWithRetry(ctx, cfg.Database.DatabaseURL())
	if err != nil {
		logger.Fatal(ctx).Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	if err := database.ApplyMigrations(ctx, db.Pool, "migrations"); err != nil {
		logger.Fatal(ctx).Err(err).Msg("failed to apply payment migrations")
	}

	service := payment.NewService(payment.NewRepository(db.Pool))
	r := chi.NewRouter()
	r.Get("/health/live", health.LiveHandler().ServeHTTP)
	r.Get("/health/ready", health.ReadyHandler(db.Health).ServeHTTP)
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
