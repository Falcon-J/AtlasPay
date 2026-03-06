package inventory

import (
	"context"

	"github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/logger"
)

// Service handles inventory business logic
type Service struct {
	repo *Repository
}

// NewService creates a new inventory service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// GetItem retrieves inventory item by SKU
func (s *Service) GetItem(ctx context.Context, sku string) (*InventoryItem, error) {
	item, err := s.repo.GetBySKU(ctx, sku)
	if err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}
	if item == nil {
		return nil, errors.ErrNotFound.WithDetails("inventory item not found")
	}
	return item, nil
}

// CheckAvailability checks if items are available
func (s *Service) CheckAvailability(ctx context.Context, items []ReserveItemRequest) (bool, error) {
	for _, reqItem := range items {
		item, err := s.repo.GetBySKU(ctx, reqItem.SKU)
		if err != nil {
			return false, errors.ErrInternalServer.WithDetails(err.Error())
		}
		if item == nil {
			return false, nil
		}
		if item.AvailableQuantity() < reqItem.Quantity {
			return false, nil
		}
	}
	return true, nil
}

// ReserveStock reserves inventory for an order
func (s *Service) ReserveStock(ctx context.Context, req *ReserveRequest) ([]*Reservation, error) {
	reservations, err := s.repo.ReserveStock(ctx, req.OrderID, req.Items)
	if err != nil {
		logger.Error(ctx).Err(err).Str("order_id", req.OrderID).Msg("failed to reserve stock")
		return nil, errors.ErrInsufficientStock.WithDetails(err.Error())
	}

	logger.Info(ctx).
		Str("order_id", req.OrderID).
		Int("items_count", len(reservations)).
		Msg("stock reserved successfully")

	return reservations, nil
}

// ReleaseStock releases reserved inventory (saga compensation)
func (s *Service) ReleaseStock(ctx context.Context, orderID string) error {
	if err := s.repo.ReleaseStock(ctx, orderID); err != nil {
		logger.Error(ctx).Err(err).Str("order_id", orderID).Msg("failed to release stock")
		return errors.ErrInternalServer.WithDetails(err.Error())
	}

	logger.Info(ctx).Str("order_id", orderID).Msg("stock released successfully")
	return nil
}

// CommitStock commits reserved inventory after successful payment
func (s *Service) CommitStock(ctx context.Context, orderID string) error {
	if err := s.repo.CommitStock(ctx, orderID); err != nil {
		logger.Error(ctx).Err(err).Str("order_id", orderID).Msg("failed to commit stock")
		return errors.ErrInternalServer.WithDetails(err.Error())
	}

	logger.Info(ctx).Str("order_id", orderID).Msg("stock committed successfully")
	return nil
}

// GetReservations retrieves reservations for an order
func (s *Service) GetReservations(ctx context.Context, orderID string) ([]*Reservation, error) {
	return s.repo.GetReservationsByOrder(ctx, orderID)
}
