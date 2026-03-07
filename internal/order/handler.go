package order

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/atlaspay/platform/internal/common/auth"
	"github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/response"
	"github.com/go-chi/chi/v5"
)

// Handler handles order HTTP requests
type Handler struct {
	service *Service
}

// NewHandler creates a new order handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Routes returns the order routes
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/", h.CreateOrder)
	r.Get("/", h.ListOrders)
	r.Get("/{id}", h.GetOrder)
	r.Get("/{id}/saga", h.GetSagaState)
	r.Patch("/{id}/cancel", h.CancelOrder)

	return r
}

// CreateOrder handles order creation
// @Summary Create a new order
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateOrderRequest true "Order details"
// @Success 201 {object} OrderResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Router /orders [post]
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		errors.WriteError(w, errors.ErrUnauthorized)
		return
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("invalid request body"))
		return
	}

	if len(req.Items) == 0 {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("at least one item is required"))
		return
	}

	for _, item := range req.Items {
		if item.SKU == "" || item.Quantity < 1 {
			errors.WriteError(w, errors.ErrBadRequest.WithDetails("invalid item: SKU and quantity > 0 required"))
			return
		}
	}

	order, err := h.service.CreateOrder(r.Context(), claims.UserID, &req)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.Created(w, OrderResponse{Order: order})
}

// GetOrder handles order retrieval
// @Summary Get order by ID
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID"
// @Success 200 {object} OrderResponse
// @Failure 404 {object} errors.AppError
// @Router /orders/{id} [get]
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		errors.WriteError(w, errors.ErrUnauthorized)
		return
	}

	orderID := chi.URLParam(r, "id")
	order, err := h.service.GetOrder(r.Context(), orderID)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	// Check ownership (unless admin)
	if order.UserID != claims.UserID && claims.Role != auth.RoleAdmin {
		errors.WriteError(w, errors.ErrForbidden)
		return
	}

	response.OK(w, OrderResponse{Order: order})
}

// ListOrders handles order listing
// @Summary List user's orders
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} OrderListResponse
// @Router /orders [get]
func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		errors.WriteError(w, errors.ErrUnauthorized)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	orders, total, err := h.service.GetUserOrders(r.Context(), claims.UserID, page, pageSize)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.OK(w, OrderListResponse{
		Orders:     orders,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
	})
}

// CancelOrder handles order cancellation
// @Summary Cancel an order
// @Tags Orders
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID"
// @Success 200 {object} OrderResponse
// @Failure 400 {object} errors.AppError
// @Router /orders/{id}/cancel [patch]
func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		errors.WriteError(w, errors.ErrUnauthorized)
		return
	}

	orderID := chi.URLParam(r, "id")

	// Get order first to check ownership
	order, err := h.service.GetOrder(r.Context(), orderID)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	// Check ownership
	if order.UserID != claims.UserID && claims.Role != auth.RoleAdmin {
		errors.WriteError(w, errors.ErrForbidden)
		return
	}

	if err := h.service.CancelOrder(r.Context(), orderID); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	// Get updated order
	order, _ = h.service.GetOrder(r.Context(), orderID)
	response.OK(w, OrderResponse{Order: order})
}

// GetSagaState handles saga state retrieval
func (h *Handler) GetSagaState(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.UserFromContext(r.Context())
	if !ok {
		errors.WriteError(w, errors.ErrUnauthorized)
		return
	}

	orderID := chi.URLParam(r, "id")
	sg, err := h.service.GetSagaState(r.Context(), orderID)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.OK(w, sg)
}
