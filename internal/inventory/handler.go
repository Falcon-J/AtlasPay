package inventory

import (
	"encoding/json"
	"net/http"

	"github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/response"
	"github.com/go-chi/chi/v5"
)

// Handler handles inventory HTTP requests
type Handler struct {
	service Port
}

// NewHandler creates a new inventory handler
func NewHandler(service Port) *Handler {
	return &Handler{service: service}
}

// Routes returns the inventory routes
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/{sku}", h.GetItem)
	r.Post("/reserve", h.ReserveStock)
	r.Post("/release", h.ReleaseStock)
	r.Post("/commit", h.CommitStock)
	r.Post("/restock", h.Restock)

	return r
}

// GetItem handles inventory item retrieval
// @Summary Get inventory item by SKU
// @Tags Inventory
// @Produce json
// @Param sku path string true "Product SKU"
// @Success 200 {object} InventoryResponse
// @Failure 404 {object} errors.AppError
// @Router /inventory/{sku} [get]
func (h *Handler) GetItem(w http.ResponseWriter, r *http.Request) {
	sku := chi.URLParam(r, "sku")

	item, err := h.service.GetItem(r.Context(), sku)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.OK(w, InventoryResponse{Item: item})
}

// ReserveStock handles stock reservation
// @Summary Reserve stock for an order
// @Tags Inventory
// @Accept json
// @Produce json
// @Param request body ReserveRequest true "Reservation details"
// @Success 200 {object} ReservationResponse
// @Failure 400 {object} errors.AppError
// @Router /inventory/reserve [post]
func (h *Handler) ReserveStock(w http.ResponseWriter, r *http.Request) {
	var req ReserveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("invalid request body"))
		return
	}

	if req.OrderID == "" || len(req.Items) == 0 {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("order_id and items are required"))
		return
	}

	reservations, err := h.service.ReserveStockV2(r.Context(), &req)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.OK(w, ReservationResponse{Reservations: reservations, Success: true})
}

// ReleaseStock handles stock release (saga compensation)
// @Summary Release reserved stock
// @Tags Inventory
// @Accept json
// @Produce json
// @Param request body ReleaseRequest true "Release details"
// @Success 200 {object} response.Response
// @Router /inventory/release [post]
func (h *Handler) ReleaseStock(w http.ResponseWriter, r *http.Request) {
	var req ReleaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("invalid request body"))
		return
	}

	if req.OrderID == "" {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("order_id is required"))
		return
	}

	if err := h.service.ReleaseStock(r.Context(), req.OrderID); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.OK(w, map[string]bool{"released": true})
}

// CommitStock handles stock commit after payment
// @Summary Commit reserved stock
// @Tags Inventory
// @Accept json
// @Produce json
// @Param request body ReleaseRequest true "Commit details"
// @Success 200 {object} response.Response
// @Router /inventory/commit [post]
func (h *Handler) CommitStock(w http.ResponseWriter, r *http.Request) {
	var req ReleaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("invalid request body"))
		return
	}

	if req.OrderID == "" {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("order_id is required"))
		return
	}

	if err := h.service.CommitStock(r.Context(), req.OrderID); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.OK(w, map[string]bool{"committed": true})
}

type RestockRequest struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

// Restock handles adding inventory stock
// @Summary Restock inventory
// @Tags Inventory
// @Accept json
// @Produce json
// @Param request body RestockRequest true "Restock details"
// @Success 200 {object} response.Response
// @Router /inventory/restock [post]
func (h *Handler) Restock(w http.ResponseWriter, r *http.Request) {
	var req RestockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("invalid request body"))
		return
	}

	if req.SKU == "" || req.Quantity <= 0 {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("sku and valid quantity required"))
		return
	}

	if err := h.service.RestockItem(r.Context(), req.SKU, req.Quantity); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.OK(w, map[string]bool{"restocked": true})
}
