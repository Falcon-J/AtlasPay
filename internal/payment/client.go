package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "github.com/atlaspay/platform/internal/common/errors"
)

// Client is the gateway adapter for the standalone payment process.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a payment client with a bounded request timeout.
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

type internalProcessRequest struct {
	UserID string `json:"user_id"`
	CreatePaymentRequest
}

func (c *Client) ProcessPaymentV2(ctx context.Context, userID string, req *CreatePaymentRequest) (*Payment, error) {
	var result PaymentResponse
	err := c.do(ctx, http.MethodPost, "/internal/v1/payments/process", internalProcessRequest{
		UserID:               userID,
		CreatePaymentRequest: *req,
	}, &result)
	if err != nil {
		return result.Payment, err
	}
	return result.Payment, nil
}

// ProcessPayment satisfies the saga.PaymentService interface.
func (c *Client) ProcessPayment(ctx context.Context, orderID, userID string, amount float64, currency, method, idempotencyKey string) error {
	_, err := c.ProcessPaymentV2(ctx, userID, &CreatePaymentRequest{
		OrderID:        orderID,
		Amount:         amount,
		Currency:       currency,
		PaymentMethod:  method,
		IdempotencyKey: idempotencyKey,
	})
	return err
}

func (c *Client) GetPayment(ctx context.Context, id string) (*Payment, error) {
	var result PaymentResponse
	if err := c.do(ctx, http.MethodGet, "/internal/v1/payments/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return result.Payment, nil
}

func (c *Client) RefundPayment(ctx context.Context, id string, reason string) (*Payment, error) {
	var result PaymentResponse
	if err := c.do(ctx, http.MethodPost, "/internal/v1/payments/"+url.PathEscape(id)+"/refund", RefundRequest{Reason: reason}, &result); err != nil {
		return result.Payment, err
	}
	return result.Payment, nil
}

var _ Port = (*Client)(nil)
