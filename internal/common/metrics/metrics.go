package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP request metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Current number of HTTP requests being processed",
	})

	// Order metrics
	ordersTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_total",
			Help: "Total number of orders",
		},
		[]string{"status"},
	)

	orderValue = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "order_value_usd",
		Help:    "Order value distribution in USD",
		Buckets: []float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
	})

	// Payment metrics
	paymentsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_total",
			Help: "Total number of payments",
		},
		[]string{"status", "method"},
	)

	paymentProcessingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "payment_processing_duration_seconds",
		Help:    "Payment processing time",
		Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5, 10},
	})

	// Saga metrics
	sagasTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sagas_total",
			Help: "Total number of sagas executed",
		},
		[]string{"name", "status"},
	)

	sagaDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "saga_duration_seconds",
			Help:    "Saga execution duration",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"name"},
	)

	sagaCompensationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "saga_compensations_total",
			Help: "Total number of saga compensations",
		},
		[]string{"name", "step"},
	)

	// Database metrics
	dbConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_active",
		Help: "Number of active database connections",
	})

	dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation"},
	)

	// Cache metrics
	cacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Total number of cache hits",
	})

	cacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "Total number of cache misses",
	})

	// Kafka metrics
	kafkaMessagesProduced = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_produced_total",
			Help: "Total number of Kafka messages produced",
		},
		[]string{"topic"},
	)

	kafkaMessagesConsumed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_consumed_total",
			Help: "Total number of Kafka messages consumed",
		},
		[]string{"topic", "group"},
	)

	kafkaEventProcessingAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_event_processing_attempts_total",
			Help: "Total number of Kafka event processing attempts",
		},
		[]string{"topic", "event_type", "status"},
	)

	kafkaEventRetries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_event_retries_total",
			Help: "Total number of Kafka event processing retries",
		},
		[]string{"topic", "event_type"},
	)

	deadLetterEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dead_letter_events_total",
			Help: "Total number of events written to dead-letter storage",
		},
		[]string{"topic", "event_type"},
	)

	kafkaConsumerLag = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kafka_consumer_lag",
			Help: "Kafka consumer lag",
		},
		[]string{"topic", "group", "partition"},
	)

	// Circuit breaker metrics
	circuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
		},
		[]string{"service"},
	)
)

// Handler returns the Prometheus metrics handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordHTTPRequest records HTTP request metrics
func RecordHTTPRequest(method, path string, status int, duration time.Duration) {
	httpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
	httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// RecordHTTPInFlight increments/decrements in-flight requests
func RecordHTTPInFlight(delta float64) {
	httpRequestsInFlight.Add(delta)
}

// RecordOrder records an order
func RecordOrder(status string, value float64) {
	ordersTotal.WithLabelValues(status).Inc()
	orderValue.Observe(value)
}

// RecordPayment records a payment
func RecordPayment(status, method string, duration time.Duration) {
	paymentsTotal.WithLabelValues(status, method).Inc()
	paymentProcessingDuration.Observe(duration.Seconds())
}

// RecordSaga records saga execution
func RecordSaga(name, status string, duration time.Duration) {
	sagasTotal.WithLabelValues(name, status).Inc()
	sagaDuration.WithLabelValues(name).Observe(duration.Seconds())
}

// RecordSagaCompensation records saga compensation
func RecordSagaCompensation(sagaName, stepName string) {
	sagaCompensationsTotal.WithLabelValues(sagaName, stepName).Inc()
}

// RecordDBConnections records database connection count
func RecordDBConnections(count int) {
	dbConnectionsActive.Set(float64(count))
}

// RecordDBQuery records database query
func RecordDBQuery(operation string, duration time.Duration) {
	dbQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordCacheHit records a cache hit
func RecordCacheHit() {
	cacheHits.Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss() {
	cacheMisses.Inc()
}

// RecordKafkaProduced records Kafka message production
func RecordKafkaProduced(topic string) {
	kafkaMessagesProduced.WithLabelValues(topic).Inc()
}

// RecordKafkaConsumed records Kafka message consumption
func RecordKafkaConsumed(topic, group string) {
	kafkaMessagesConsumed.WithLabelValues(topic, group).Inc()
}

func RecordKafkaEventAttempt(topic, eventType, status string) {
	kafkaEventProcessingAttempts.WithLabelValues(topic, eventType, status).Inc()
}

func RecordKafkaRetry(topic, eventType string) {
	kafkaEventRetries.WithLabelValues(topic, eventType).Inc()
}

func RecordDeadLetterEvent(topic, eventType string) {
	deadLetterEvents.WithLabelValues(topic, eventType).Inc()
}

// RecordKafkaLag records Kafka consumer lag
func RecordKafkaLag(topic, group, partition string, lag int64) {
	kafkaConsumerLag.WithLabelValues(topic, group, partition).Set(float64(lag))
}

// RecordCircuitBreakerState records circuit breaker state
func RecordCircuitBreakerState(service string, state int) {
	circuitBreakerState.WithLabelValues(service).Set(float64(state))
}
