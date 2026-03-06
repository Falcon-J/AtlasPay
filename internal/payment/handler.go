package payment

import (
	"encoding/json"
	"net/http"

	"github.com/atlaspay/platform/internal/common/auth"
	"github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/response"
	"github.com/go-chi/chi/v5"
)

// Handler handles payment HTTP requests
type Handler struct {
	service *Service
}

// NewHandler creates a new payment handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Routes returns the payment routes
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/", h.ProcessPayment)
	r.Get("/{id}", h.GetPayment)
	r.Post("/{id}/refund", h.RefundPayment)

	return r
}

// ProcessPayment handles payment processing
// @Summary Process a payment
// @Tags Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreatePaymentRequest true "Payment details"
// @Success 201 {object} PaymentResponse
// @Failure 400 {object} errors.AppError
// @Router /payments [post]
func (h *Handler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		errors.WriteError(w, errors.ErrUnauthorized)
		return
	}

	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("invalid request body"))
		return
	}

	if req.OrderID == "" || req.Amount <= 0 || req.IdempotencyKey == "" {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("order_id, amount, and idempotency_key are required"))
		return
	}

	payment, err := h.service.ProcessPayment(r.Context(), claims.UserID, &req)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			// For payment failures, still return the payment object
			if payment != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(appErr.Code)
				json.NewEncoder(w).Encode(PaymentResponse{Payment: payment})
				return
			}
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.Created(w, PaymentResponse{Payment: payment})
}

// GetPayment handles payment retrieval
// @Summary Get payment by ID
// @Tags Payments
// @Produce json
// @Security BearerAuth
// @Param id path string true "Payment ID"
// @Success 200 {object} PaymentResponse
// @Failure 404 {object} errors.AppError
// @Router /payments/{id} [get]
func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		errors.WriteError(w, errors.ErrUnauthorized)
		return
	}

	paymentID := chi.URLParam(r, "id")
	payment, err := h.service.GetPayment(r.Context(), paymentID)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	// Check ownership
	if payment.UserID != claims.UserID && claims.Role != auth.RoleAdmin {
		errors.WriteError(w, errors.ErrForbidden)
		return
	}

	response.OK(w, PaymentResponse{Payment: payment})
}

// RefundPayment handles payment refund
// @Summary Refund a payment
// @Tags Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Payment ID"
// @Param request body RefundRequest true "Refund reason"
// @Success 200 {object} PaymentResponse
// @Failure 400 {object} errors.AppError
// @Router /payments/{id}/refund [post]
func (h *Handler) RefundPayment(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		errors.WriteError(w, errors.ErrUnauthorized)
		return
	}

	// Only admins can refund
	if claims.Role != auth.RoleAdmin {
		errors.WriteError(w, errors.ErrForbidden.WithDetails("only admins can process refunds"))
		return
	}

	paymentID := chi.URLParam(r, "id")

	var req RefundRequest
	json.NewDecoder(r.Body).Decode(&req)

	payment, err := h.service.RefundPayment(r.Context(), paymentID, req.Reason)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.OK(w, PaymentResponse{Payment: payment})
}
