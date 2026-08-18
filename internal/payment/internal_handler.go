package payment

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	apperrors "github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/metrics"
	"github.com/atlaspay/platform/internal/common/response"
	"github.com/go-chi/chi/v5"
)

// NewInternalHandler exposes the narrow service-to-service payment contract.
// Authentication is owned by the gateway; this endpoint is only published on
// the private Compose network and should not be exposed directly in production.
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
	r.Handle("/metrics", metrics.Handler())
	r.Post("/internal/v1/payments/process", func(w http.ResponseWriter, r *http.Request) {
		var req internalProcessRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" ||
			req.OrderID == "" || req.Amount <= 0 || req.Currency == "" ||
			req.PaymentMethod == "" || req.IdempotencyKey == "" {
			apperrors.WriteError(w, apperrors.ErrBadRequest.WithDetails("user_id and payment fields are required"))
			return
		}

		payment, err := service.ProcessPaymentV2(r.Context(), req.UserID, &req.CreatePaymentRequest)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				if payment != nil {
					response.JSON(w, appErr.Code, PaymentResponse{Payment: payment})
				} else {
					apperrors.WriteError(w, appErr)
				}
				return
			}
			apperrors.WriteError(w, apperrors.ErrInternalServer)
			return
		}
		response.Created(w, PaymentResponse{Payment: payment})
	})

	r.Get("/internal/v1/payments/{id}", func(w http.ResponseWriter, r *http.Request) {
		payment, err := service.GetPayment(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeInternalError(w, err)
			return
		}
		response.OK(w, PaymentResponse{Payment: payment})
	})

	r.Post("/internal/v1/payments/{id}/refund", func(w http.ResponseWriter, r *http.Request) {
		var req RefundRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, apperrors.ErrBadRequest.WithDetails("invalid refund request"))
			return
		}
		payment, err := service.RefundPayment(r.Context(), chi.URLParam(r, "id"), req.Reason)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		response.OK(w, PaymentResponse{Payment: payment})
	})
	return r
}

func writeInternalError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		apperrors.WriteError(w, appErr)
		return
	}
	apperrors.WriteError(w, apperrors.ErrInternalServer)
}
