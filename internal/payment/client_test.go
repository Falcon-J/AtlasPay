package payment

import (
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/response"
)

func TestClientProcessesPaymentThroughContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") != "test-token" {
			apperrors.WriteError(w, apperrors.ErrUnauthorized)
			return
		}
		response.Created(w, PaymentResponse{Payment: &Payment{
			ID:     "pay-1",
			Status: PaymentCompleted,
		}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	result, err := client.ProcessPaymentV2(t.Context(), "user-1", &CreatePaymentRequest{
		OrderID:        "order-1",
		Amount:         10,
		Currency:       "USD",
		PaymentMethod:  "test-card",
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatalf("ProcessPaymentV2 returned error: %v", err)
	}
	if result == nil || result.ID != "pay-1" || result.Status != PaymentCompleted {
		t.Fatalf("unexpected payment result: %#v", result)
	}
}

func TestClientPreservesPaymentFailureResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusPaymentRequired, PaymentResponse{Payment: &Payment{
			ID:            "pay-failed",
			Status:        PaymentFailed,
			FailureReason: "declined",
		}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	result, err := client.ProcessPaymentV2(t.Context(), "user-1", &CreatePaymentRequest{
		OrderID:        "order-1",
		Amount:         10,
		Currency:       "USD",
		PaymentMethod:  "test-card",
		IdempotencyKey: "idem-1",
	})
	if err == nil {
		t.Fatal("expected payment failure")
	}
	if result == nil || result.Status != PaymentFailed {
		t.Fatalf("expected failed payment in error response: %#v", result)
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != http.StatusPaymentRequired {
		t.Fatalf("expected payment-required app error, got %T %v", err, err)
	}
}
