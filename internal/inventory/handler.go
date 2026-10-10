package inventory

import (
	"encoding/json"
	"net/http"

	"github.com/atlaspay/platform/internal/common/auth"
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
	r.Post("/restock", h.Restock)

	return r
}

// GetItem handles inventory item retrieval
// @Summary Get inventory item by SKU
// @Tags Inventory
// @Produce json
// @Security BearerAuth
// @Param sku path string true "Product SKU"
// @Success 200 {object} InventoryResponse
// @Failure 401 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /inventory/{sku} [get]
func (h *Handler) GetItem(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserFromContext(r.Context()); !ok {
		errors.WriteError(w, errors.ErrUnauthorized)
		return
	}

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

type RestockRequest struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

// Restock handles adding inventory stock
// @Summary Restock inventory
// @Tags Inventory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body RestockRequest true "Restock details"
// @Success 200 {object} response.Response
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Router /inventory/restock [post]
func (h *Handler) Restock(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		errors.WriteError(w, errors.ErrUnauthorized)
		return
	}
	if claims.Role != auth.RoleAdmin {
		errors.WriteError(w, errors.ErrForbidden)
		return
	}

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
