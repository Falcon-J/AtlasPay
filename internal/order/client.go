package order

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	commonauth "github.com/atlaspay/platform/internal/common/auth"
	apperrors "github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/saga"
)

// Client is the gateway adapter for the standalone order process.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type wireResponse struct {
	Success bool                `json:"success"`
	Data    json.RawMessage     `json:"data"`
	Meta    json.RawMessage     `json:"meta"`
	Error   *apperrors.AppError `json:"error"`
}

func (c *Client) do(ctx context.Context, method, path string, userID string, role commonauth.Role, body interface{}, result interface{}) (*wireResponse, error) {
	var requestBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&requestBody).Encode(body); err != nil {
			return nil, apperrors.ErrInternalServer.WithDetails(err.Error())
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &requestBody)
	if err != nil {
		return nil, apperrors.ErrServiceUnavailable.WithDetails(err.Error())
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("X-Internal-Token", c.token)
	}
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
		req.Header.Set("X-User-Role", string(role))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperrors.ErrServiceUnavailable.WithDetails(err.Error())
	}
	defer resp.Body.Close()

	var wire wireResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, apperrors.ErrServiceUnavailable.WithDetails(err.Error())
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if wire.Error != nil {
			return &wire, wire.Error
		}
		return &wire, apperrors.New(resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	if result != nil && len(wire.Data) > 0 {
		if err := json.Unmarshal(wire.Data, result); err != nil {
			return nil, apperrors.ErrServiceUnavailable.WithDetails(err.Error())
		}
	}
	return &wire, nil
}

func (c *Client) CreateOrder(ctx context.Context, userID string, req *CreateOrderRequest) (*Order, error) {
	var result OrderResponse
	_, err := c.do(ctx, http.MethodPost, "/internal/v1/orders", userID, commonauth.RoleUser, req, &result)
	return result.Order, err
}

func (c *Client) GetOrder(ctx context.Context, id string) (*Order, error) {
	var result OrderResponse
	userID, role := userFromContext(ctx)
	_, err := c.do(ctx, http.MethodGet, "/internal/v1/orders/"+url.PathEscape(id), userID, role, nil, &result)
	return result.Order, err
}

func (c *Client) GetUserOrders(ctx context.Context, userID string, page, pageSize int) ([]*Order, int64, error) {
	path := "/internal/v1/orders?page=" + strconv.Itoa(page) + "&page_size=" + strconv.Itoa(pageSize)
	var result OrderListResponse
	wire, err := c.do(ctx, http.MethodGet, path, userID, commonauth.RoleUser, nil, &result)
	if err != nil {
		return nil, 0, err
	}
	var meta struct {
		Total int64 `json:"total"`
	}
	if len(wire.Meta) > 0 {
		_ = json.Unmarshal(wire.Meta, &meta)
	}
	return result.Orders, meta.Total, nil
}

func (c *Client) CancelOrder(ctx context.Context, id string) error {
	userID, role := userFromContext(ctx)
	_, err := c.do(ctx, http.MethodPatch, "/internal/v1/orders/"+url.PathEscape(id)+"/cancel", userID, role, nil, nil)
	return err
}

func (c *Client) GetSagaState(ctx context.Context, orderID string) (*saga.Saga, error) {
	var result saga.Saga
	userID, role := userFromContext(ctx)
	_, err := c.do(ctx, http.MethodGet, "/internal/v1/orders/"+url.PathEscape(orderID)+"/saga", userID, role, nil, &result)
	return &result, err
}

func userFromContext(ctx context.Context) (string, commonauth.Role) {
	claims, ok := commonauth.UserFromContext(ctx)
	if !ok {
		return "", commonauth.RoleUser
	}
	return claims.UserID, claims.Role
}

var _ Port = (*Client)(nil)
