package inventory

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"

	apperrors "github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/metrics"
	"github.com/atlaspay/platform/internal/common/response"
	"github.com/go-chi/chi/v5"
)

// NewInternalHandler exposes the private inventory contract used by the
// gateway and checkout saga.
func NewInternalHandler(service *Service, token string) chi.Router {
	r := chi.NewRouter()
	if token != "" {
		r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/metrics" {
				next.ServeHTTP(w, req)
				return
			}
			provided := []byte(req.Header.Get("X-Internal-Token"))
				expected := []byte(token)
				if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
					apperrors.WriteError(w, apperrors.ErrUnauthorized)
					return
				}
				next.ServeHTTP(w, req)
			})
		})
	}
	r.Handle("/metrics", metrics.Handler())

	r.Get("/internal/v1/inventory/{sku}", func(w http.ResponseWriter, req *http.Request) {
		item, err := service.GetItem(req.Context(), chi.URLParam(req, "sku"))
		if err != nil {
			writeInternalError(w, err)
			return
		}
		response.OK(w, InventoryResponse{Item: item})
	})
	r.Post("/internal/v1/inventory/availability", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Items []ReserveItemRequest `json:"items"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || len(body.Items) == 0 {
			apperrors.WriteError(w, apperrors.ErrBadRequest.WithDetails("items are required"))
			return
		}
		available, err := service.CheckAvailability(req.Context(), body.Items)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		response.OK(w, map[string]bool{"available": available})
	})
	r.Post("/internal/v1/inventory/reserve", func(w http.ResponseWriter, req *http.Request) {
		var body ReserveRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.OrderID == "" || len(body.Items) == 0 {
			apperrors.WriteError(w, apperrors.ErrBadRequest.WithDetails("order_id and items are required"))
			return
		}
		reservations, err := service.ReserveStockV2(req.Context(), &body)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		response.OK(w, ReservationResponse{Reservations: reservations, Success: true})
	})
	r.Post("/internal/v1/inventory/release", func(w http.ResponseWriter, req *http.Request) {
		mutateInventory(w, req, service.ReleaseStock)
	})
	r.Post("/internal/v1/inventory/commit", func(w http.ResponseWriter, req *http.Request) {
		mutateInventory(w, req, service.CommitStock)
	})
	r.Get("/internal/v1/inventory/reservations/{orderID}", func(w http.ResponseWriter, req *http.Request) {
		reservations, err := service.GetReservations(req.Context(), chi.URLParam(req, "orderID"))
		if err != nil {
			writeInternalError(w, err)
			return
		}
		response.OK(w, ReservationResponse{Reservations: reservations, Success: true})
	})
	r.Post("/internal/v1/inventory/restock", func(w http.ResponseWriter, req *http.Request) {
		var body RestockRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.SKU == "" || body.Quantity <= 0 {
			apperrors.WriteError(w, apperrors.ErrBadRequest.WithDetails("sku and valid quantity are required"))
			return
		}
		if err := service.RestockItem(req.Context(), body.SKU, body.Quantity); err != nil {
			writeInternalError(w, err)
			return
		}
		response.OK(w, map[string]bool{"restocked": true})
	})
	return r
}

func mutateInventory(w http.ResponseWriter, req *http.Request, mutate func(context.Context, string) error) {
	var body ReleaseRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.OrderID == "" {
		apperrors.WriteError(w, apperrors.ErrBadRequest.WithDetails("order_id is required"))
		return
	}
	if err := mutate(req.Context(), body.OrderID); err != nil {
		writeInternalError(w, err)
		return
	}
	response.OK(w, map[string]bool{"ok": true})
}

func writeInternalError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		apperrors.WriteError(w, appErr)
		return
	}
	apperrors.WriteError(w, apperrors.ErrInternalServer)
}
