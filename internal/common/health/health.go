package health

import (
	"context"
	"encoding/json"
	"net/http"
)

// Check is a dependency check used by the readiness endpoint.
type Check func(context.Context) error

// LiveHandler reports only that the process is able to serve HTTP requests.
// It deliberately does not call external dependencies.
func LiveHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeStatus(w, http.StatusOK, "alive")
	})
}

// ReadyHandler reports whether all supplied dependencies are available.
func ReadyHandler(checks ...Check) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, check := range checks {
			if check != nil && check(r.Context()) != nil {
				writeStatus(w, http.StatusServiceUnavailable, "not_ready")
				return
			}
		}
		writeStatus(w, http.StatusOK, "ready")
	})
}

func writeStatus(w http.ResponseWriter, status int, value string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": value})
}
