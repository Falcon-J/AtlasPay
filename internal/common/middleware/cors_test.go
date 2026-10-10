package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORS(t *testing.T) {
	for _, tc := range []struct {
		name, origin, method, requestedMethod, headers string
		allowed                                        []string
		status                                         int
		called                                         bool
	}{
		{"no origin", "", "GET", "", "", nil, 200, true},
		{"same origin", "http://api.example", "POST", "", "", nil, 200, true},
		{"empty list blocks cross origin", "https://web.example", "POST", "", "", nil, 403, false},
		{"exact allowed", "https://web.example", "POST", "", "", []string{"https://web.example"}, 200, true},
		{"no suffix matching", "https://web.example.attacker", "POST", "", "", []string{"https://web.example"}, 403, false},
		{"preflight", "https://web.example", "OPTIONS", "POST", "authorization, Content-Type", []string{"https://web.example"}, 204, false},
		{"bad method", "https://web.example", "OPTIONS", "TRACE", "", []string{"https://web.example"}, 403, false},
		{"bad header", "https://web.example", "OPTIONS", "POST", "X-Service-Token", []string{"https://web.example"}, 403, false},
		{"ordinary options", "", "OPTIONS", "", "", nil, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := CORS(tc.allowed)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(200) }))
			r := httptest.NewRequest(tc.method, "http://api.example/api/orders", nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Access-Control-Request-Method", tc.requestedMethod)
			r.Header.Set("Access-Control-Request-Headers", tc.headers)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || called != tc.called {
				t.Fatalf("status=%d called=%v", w.Code, called)
			}
			if !strings.Contains(strings.Join(w.Header().Values("Vary"), ","), "Origin") {
				t.Fatal("missing Vary Origin")
			}
			if tc.status < 300 && tc.origin != "" && w.Header().Get("Access-Control-Allow-Origin") != tc.origin {
				t.Fatal("missing exact allow origin")
			}
			if tc.status == 403 && w.Header().Get("Access-Control-Allow-Methods") != "" {
				t.Fatal("rejected preflight was granted methods")
			}
		})
	}
}
