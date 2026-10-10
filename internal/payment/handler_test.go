package payment

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/atlaspay/platform/internal/common/auth"
)

type handlerPayment struct {
	Port
	calls int
}

func (s *handlerPayment) ProcessPaymentV2(context.Context, string, *CreatePaymentRequest) (*Payment, error) {
	s.calls++
	return &Payment{ID: "unexpected-payment"}, nil
}

func TestPublicPaymentCreationDisabled(t *testing.T) {
	for _, tc := range []struct {
		name   string
		claims *auth.Claims
		status int
	}{
		{"anonymous", nil, 401},
		{"user", &auth.Claims{UserID: "user-1", Role: auth.RoleUser}, 405},
		{"admin", &auth.Claims{UserID: "admin", Role: auth.RoleAdmin}, 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &handlerPayment{}
			request := httptest.NewRequest("POST", "/", strings.NewReader(`{"order_id":"order-1","amount":10,"idempotency_key":"key-1"}`))
			if tc.claims != nil {
				request = request.WithContext(auth.ContextWithUser(request.Context(), tc.claims))
			}
			response := httptest.NewRecorder()
			NewHandler(service).Routes().ServeHTTP(response, request)
			if response.Code != tc.status || service.calls != 0 {
				t.Fatalf("status=%d calls=%d; want status=%d calls=0", response.Code, service.calls, tc.status)
			}
		})
	}
}
