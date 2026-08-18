package order

import (
	"context"

	"github.com/atlaspay/platform/internal/common/saga"
)

// Port is the public order capability used by the gateway. Service and Client
// are the local and remote adapters at this process boundary.
type Port interface {
	CreateOrder(ctx context.Context, userID string, req *CreateOrderRequest) (*Order, error)
	GetOrder(ctx context.Context, id string) (*Order, error)
	GetUserOrders(ctx context.Context, userID string, page, pageSize int) ([]*Order, int64, error)
	CancelOrder(ctx context.Context, id string) error
	GetSagaState(ctx context.Context, orderID string) (*saga.Saga, error)
}
