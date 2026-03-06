package logger

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
)

type ctxKey string

const correlationIDKey ctxKey = "correlation_id"

var log zerolog.Logger

func init() {
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	log = zerolog.New(output).With().Timestamp().Caller().Logger()
}

// Init initializes the logger with the given service name
func Init(serviceName string) {
	log = log.With().Str("service", serviceName).Logger()
}

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// GetCorrelationID retrieves the correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// Info logs an info message
func Info(ctx context.Context) *zerolog.Event {
	return log.Info().Str("correlation_id", GetCorrelationID(ctx))
}

// Error logs an error message
func Error(ctx context.Context) *zerolog.Event {
	return log.Error().Str("correlation_id", GetCorrelationID(ctx))
}

// Debug logs a debug message
func Debug(ctx context.Context) *zerolog.Event {
	return log.Debug().Str("correlation_id", GetCorrelationID(ctx))
}

// Warn logs a warning message
func Warn(ctx context.Context) *zerolog.Event {
	return log.Warn().Str("correlation_id", GetCorrelationID(ctx))
}

// Fatal logs a fatal message and exits
func Fatal(ctx context.Context) *zerolog.Event {
	return log.Fatal().Str("correlation_id", GetCorrelationID(ctx))
}

// WithField returns a new event with the given field
func WithField(ctx context.Context, key string, value interface{}) *zerolog.Event {
	return log.Info().Str("correlation_id", GetCorrelationID(ctx)).Interface(key, value)
}
