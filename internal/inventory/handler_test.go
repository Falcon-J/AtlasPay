package inventory

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/atlaspay/platform/internal/common/auth"
)

type handlerInventory struct {
	Port
	item  InventoryItem
	calls int
}

func (s *handlerInventory) GetItem(context.Context, string) (*InventoryItem, error) {
	s.calls++
	return &s.item, nil
}

func (s *handlerInventory) RestockItem(_ context.Context, _ string, quantity int) error {
	s.calls++
	s.item.Quantity += quantity
	return nil
}

func (s *handlerInventory) ReserveStockV2(context.Context, *ReserveRequest) ([]*Reservation, error) {
	s.calls++
	s.item.ReservedQty++
	return nil, nil
}

func (s *handlerInventory) ReleaseStock(context.Context, string) error {
	s.calls++
	s.item.ReservedQty--
	return nil
}

func (s *handlerInventory) CommitStock(context.Context, string) error {
	s.calls++
	s.item.Quantity--
	return nil
}

func TestPublicInventoryAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body string
		claims                   *auth.Claims
		status, calls, quantity  int
	}{
		{"anonymous read", "GET", "/LAPTOP-001", "", nil, 401, 0, 10},
		{"user read", "GET", "/LAPTOP-001", "", &auth.Claims{UserID: "user-1", Role: auth.RoleUser}, 200, 1, 10},
		{"anonymous restock", "POST", "/restock", `{"sku":"LAPTOP-001","quantity":3}`, nil, 401, 0, 10},
		{"user restock", "POST", "/restock", `{"sku":"LAPTOP-001","quantity":3}`, &auth.Claims{UserID: "user-1", Role: auth.RoleUser}, 403, 0, 10},
		{"second user restock", "POST", "/restock", `{"sku":"LAPTOP-001","quantity":3}`, &auth.Claims{UserID: "user-2", Role: auth.RoleUser}, 403, 0, 10},
		{"admin restock", "POST", "/restock", `{"sku":"LAPTOP-001","quantity":3}`, &auth.Claims{UserID: "admin", Role: auth.RoleAdmin}, 200, 1, 13},
		{"user reserve", "POST", "/reserve", `{"order_id":"order-1","items":[{"sku":"LAPTOP-001","quantity":1}]}`, &auth.Claims{UserID: "user-1", Role: auth.RoleUser}, 405, 0, 10},
		{"user release", "POST", "/release", `{"order_id":"order-1"}`, &auth.Claims{UserID: "user-1", Role: auth.RoleUser}, 405, 0, 10},
		{"admin commit", "POST", "/commit", `{"order_id":"order-1"}`, &auth.Claims{UserID: "admin", Role: auth.RoleAdmin}, 405, 0, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &handlerInventory{item: InventoryItem{Quantity: 10, ReservedQty: 2}}
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.claims != nil {
				request = request.WithContext(auth.ContextWithUser(request.Context(), tc.claims))
			}
			response := httptest.NewRecorder()
			NewHandler(service).Routes().ServeHTTP(response, request)
			if response.Code != tc.status || service.calls != tc.calls || service.item.Quantity != tc.quantity || service.item.ReservedQty != 2 {
				t.Fatalf("status=%d calls=%d inventory=%+v; want status=%d calls=%d quantity=%d reserved=2", response.Code, service.calls, service.item, tc.status, tc.calls, tc.quantity)
			}
		})
	}
}
