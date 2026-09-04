package order

import (
	"crypto/subtle"
	"net/http"

	commonauth "github.com/atlaspay/platform/internal/common/auth"
	apperrors "github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/metrics"
	"github.com/go-chi/chi/v5"
)

// NewInternalHandler exposes the order HTTP contract to the gateway. The
// gateway remains the public JWT boundary and forwards only the authenticated
// user identity and role over this private network contract.
func NewInternalHandler(service *Service, token string) chi.Router {
	r := chi.NewRouter()
	if token != "" {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/metrics" {
					next.ServeHTTP(w, r)
					return
				}
				provided := []byte(r.Header.Get("X-Internal-Token"))
				expected := []byte(token)
				if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
					apperrors.WriteError(w, apperrors.ErrUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
	}

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}
			userID := r.Header.Get("X-User-ID")
			if userID == "" {
				apperrors.WriteError(w, apperrors.ErrUnauthorized)
				return
			}
			role := commonauth.Role(r.Header.Get("X-User-Role"))
			if role != commonauth.RoleAdmin {
				role = commonauth.RoleUser
			}
			claims := &commonauth.Claims{UserID: userID, Role: role}
			next.ServeHTTP(w, r.WithContext(commonauth.ContextWithUser(r.Context(), claims)))
		})
	})

	r.Handle("/metrics", metrics.Handler())
	r.Mount("/internal/v1/orders", NewHandler(service).Routes())
	return r
}
