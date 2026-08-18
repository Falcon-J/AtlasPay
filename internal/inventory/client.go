package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/saga"
)

// Client is the gateway adapter for the standalone inventory process.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type wireResponse struct {
	Success bool                `json:"success"`
	Data    json.RawMessage     `json:"data"`
	Error   *apperrors.AppError `json:"error"`
}

func (c *Client) do(ctx context.Context, method, path string, requestBody interface{}, result interface{}) error {
	var body bytes.Buffer
	if requestBody != nil {
		if err := json.NewEncoder(&body).Encode(requestBody); err != nil {
			return apperrors.ErrInternalServer.WithDetails(err.Error())
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &body)
	if err != nil {
		return apperrors.ErrServiceUnavailable.WithDetails(err.Error())
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("X-Internal-Token", c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apperrors.ErrServiceUnavailable.WithDetails(err.Error())
	}
	defer resp.Body.Close()

	var wire wireResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return apperrors.ErrServiceUnavailable.WithDetails(err.Error())
	}
	if result != nil && len(wire.Data) > 0 {
		if err := json.Unmarshal(wire.Data, result); err != nil {
			return apperrors.ErrServiceUnavailable.WithDetails(err.Error())
		}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if wire.Error != nil {
			return wire.Error
		}
		return apperrors.New(resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return nil
}

func (c *Client) GetItem(ctx context.Context, sku string) (*InventoryItem, error) {
	var result InventoryResponse
	if err := c.do(ctx, http.MethodGet, "/internal/v1/inventory/"+url.PathEscape(sku), nil, &result); err != nil {
		return nil, err
	}
	return result.Item, nil
}

func (c *Client) CheckAvailability(ctx context.Context, items []ReserveItemRequest) (bool, error) {
	var result struct {
		Available bool `json:"available"`
	}
	if err := c.do(ctx, http.MethodPost, "/internal/v1/inventory/availability", map[string]interface{}{"items": items}, &result); err != nil {
		return false, err
	}
	return result.Available, nil
}

func (c *Client) ReserveStockV2(ctx context.Context, req *ReserveRequest) ([]*Reservation, error) {
	var result ReservationResponse
	if err := c.do(ctx, http.MethodPost, "/internal/v1/inventory/reserve", req, &result); err != nil {
		return nil, err
	}
	return result.Reservations, nil
}

func (c *Client) ReserveStock(ctx context.Context, orderID string, items []saga.OrderItem) error {
	reqItems := make([]ReserveItemRequest, len(items))
	for i, item := range items {
		reqItems[i] = ReserveItemRequest{SKU: item.SKU, Quantity: item.Quantity}
	}
	_, err := c.ReserveStockV2(ctx, &ReserveRequest{OrderID: orderID, Items: reqItems})
	return err
}

func (c *Client) ReleaseStock(ctx context.Context, orderID string) error {
	return c.mutate(ctx, "/internal/v1/inventory/release", orderID)
}

func (c *Client) CommitStock(ctx context.Context, orderID string) error {
	return c.mutate(ctx, "/internal/v1/inventory/commit", orderID)
}

func (c *Client) mutate(ctx context.Context, path, orderID string) error {
	return c.do(ctx, http.MethodPost, path, ReleaseRequest{OrderID: orderID}, nil)
}

func (c *Client) GetReservations(ctx context.Context, orderID string) ([]*Reservation, error) {
	var result ReservationResponse
	if err := c.do(ctx, http.MethodGet, "/internal/v1/inventory/reservations/"+url.PathEscape(orderID), nil, &result); err != nil {
		return nil, err
	}
	return result.Reservations, nil
}

func (c *Client) RestockItem(ctx context.Context, sku string, quantity int) error {
	return c.do(ctx, http.MethodPost, "/internal/v1/inventory/restock", RestockRequest{SKU: sku, Quantity: quantity}, nil)
}

var _ Port = (*Client)(nil)
