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
	"github.com/atlaspay/platform/internal/common/dlq"
	"github.com/atlaspay/platform/internal/common/health"
	"github.com/atlaspay/platform/internal/common/kafka"
	"github.com/atlaspay/platform/internal/common/logger"
	"github.com/atlaspay/platform/internal/common/response"
	"github.com/atlaspay/platform/internal/inventory"
	"github.com/atlaspay/platform/internal/order"
	"github.com/atlaspay/platform/internal/payment"
	"github.com/go-chi/chi/v5"
)

func main() {
	logger.Init("order-service")
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	db, err := database.ConnectWithRetry(ctx, cfg.Database.DatabaseURL())
	if err != nil {
		logger.Fatal(ctx).Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	if err := database.ApplyMigrations(ctx, db.Pool, "migrations"); err != nil {
		logger.Fatal(ctx).Err(err).Msg("failed to apply database migrations")
	}

	redisCache, err := cache.NewRedisCache(cfg.Redis.RedisAddr(), cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		logger.Fatal(ctx).Err(err).Msg("failed to connect to Redis")
	}
	defer redisCache.Close()

	producer := kafka.NewProducer(cfg.Kafka.Brokers)
	defer producer.Close()

	paymentPort := payment.NewClient(os.Getenv("PAYMENT_SERVICE_URL"), os.Getenv("PAYMENT_SERVICE_TOKEN"))
	inventoryPort := inventory.NewClient(os.Getenv("INVENTORY_SERVICE_URL"), os.Getenv("INVENTORY_SERVICE_TOKEN"))
	service := order.NewServiceWithKafka(order.NewRepository(db.Pool, redisCache), inventoryPort, paymentPort, producer, true)
	dlqRepo := dlq.NewRepository(db.Pool)
	workerCount := cfg.Kafka.Workers
	if workerCount < 1 {
		workerCount = 1
	}
	for i := 0; i < workerCount; i++ {
		consumer := kafka.NewConsumerWithOptions(cfg.Kafka.Brokers, "atlaspay.orders", cfg.Kafka.GroupID+"-orders", service, dlqRepo, producer)
		go consumer.Start(ctx)
		defer consumer.Close()
	}
	service.StartOutboxPublisher(ctx)

	r := chi.NewRouter()
	r.Get("/health/live", health.LiveHandler().ServeHTTP)
	r.Get("/health/ready", health.ReadyHandler(db.Health, redisCache.Health).ServeHTTP)
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
	r.Mount("/", order.NewInternalHandler(service, os.Getenv("ORDER_SERVICE_TOKEN")))

	port := os.Getenv("ORDER_SERVICE_PORT")
	if port == "" {
		port = "8083"
	}
	server := &http.Server{Addr: ":" + port, Handler: r, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info(ctx).Str("port", port).Int("workers", workerCount).Msg("order service started")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal(ctx).Err(err).Msg("order service failed")
	}
}
