package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/atlaspay/platform/internal/common/auth"
	"github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/logger"
	"github.com/atlaspay/platform/internal/common/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RequestLogger logs incoming requests with correlation ID
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get or generate correlation ID
		correlationID := r.Header.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		// Add to context
		ctx := logger.WithCorrelationID(r.Context(), correlationID)
		r = r.WithContext(ctx)

		// Add to response header
		w.Header().Set("X-Correlation-ID", correlationID)

		// Wrap response writer to capture status
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		path := r.URL.Path
		if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil && routeCtx.RoutePattern() != "" {
			path = routeCtx.RoutePattern()
		}

		metrics.RecordHTTPRequest(r.Method, path, wrapped.status, duration)

		// Log request
		logger.Info(ctx).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", wrapped.status).
			Dur("duration", duration).
			Msg("request completed")
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// JWTAuth validates JWT tokens
func JWTAuth(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				errors.WriteError(w, errors.ErrUnauthorized.WithDetails("missing authorization header"))
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				errors.WriteError(w, errors.ErrUnauthorized.WithDetails("invalid authorization header format"))
				return
			}

			claims, err := jwtManager.ValidateAccessToken(parts[1])
			if err != nil {
				errors.WriteError(w, errors.ErrInvalidToken.WithDetails(err.Error()))
				return
			}

			// Add claims to context
			ctx := auth.ContextWithUser(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole checks if user has required role
func RequireRole(roles ...auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.UserFromContext(r.Context())
			if !ok {
				errors.WriteError(w, errors.ErrUnauthorized)
				return
			}

			for _, role := range roles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			errors.WriteError(w, errors.ErrForbidden.WithDetails("insufficient permissions"))
		})
	}
}

// CORS permits only configured origins and same-origin requests.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			allowed := origin == scheme+"://"+r.Host
			for _, candidate := range allowedOrigins {
				if candidate == "*" || candidate == origin {
					allowed = true
					break
				}
			}
			if !allowed {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Expose-Headers", "X-Correlation-ID")
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Add("Vary", "Access-Control-Request-Method")
				w.Header().Add("Vary", "Access-Control-Request-Headers")
				method := r.Header.Get("Access-Control-Request-Method")
				if !strings.Contains(",GET,POST,PUT,DELETE,PATCH,", ","+method+",") {
					http.Error(w, "method not allowed", http.StatusForbidden)
					return
				}
				for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
					switch strings.ToLower(strings.TrimSpace(header)) {
					case "", "content-type", "authorization", "x-correlation-id":
					default:
						http.Error(w, "header not allowed", http.StatusForbidden)
						return
					}
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Recoverer recovers from panics
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				logger.Error(r.Context()).Interface("panic", rvr).Msg("panic recovered")
				errors.WriteError(w, errors.ErrInternalServer)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Timeout adds request timeout
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
