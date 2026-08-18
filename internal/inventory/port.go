package inventory

import (
	"context"

	"github.com/atlaspay/platform/internal/common/saga"
)

// Port is the inventory capability used by public handlers and checkout.
// LocalService and Client are the two adapters at this seam.
type Port interface {
	saga.InventoryService
	GetItem(ctx context.Context, sku string) (*InventoryItem, error)
	CheckAvailability(ctx context.Context, items []ReserveItemRequest) (bool, error)
	ReserveStockV2(ctx context.Context, req *ReserveRequest) ([]*Reservation, error)
	GetReservations(ctx context.Context, orderID string) ([]*Reservation, error)
	RestockItem(ctx context.Context, sku string, quantity int) error
}
