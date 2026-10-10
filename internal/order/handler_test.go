package order

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atlaspay/platform/internal/common/auth"
	apperrors "github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/response"
	"github.com/atlaspay/platform/internal/common/saga"
)

type handlerOrder struct {
	Port
	orderErr  error
	sagaCalls int
}

func (s *handlerOrder) GetOrder(context.Context, string) (*Order, error) {
	return &Order{ID: "order-1", UserID: "owner"}, s.orderErr
}

func (s *handlerOrder) GetSagaState(context.Context, string) (*saga.Saga, error) {
	s.sagaCalls++
	return &saga.Saga{ID: "saga-1"}, nil
}

func TestSagaOwnership(t *testing.T) {
	for _, adapter := range []string{"local", "remote"} {
		for _, tc := range []struct {
			name              string
			claims            *auth.Claims
			orderErr          error
			status, sagaCalls int
		}{
			{"anonymous", nil, nil, 401, 0},
			{"owner", &auth.Claims{UserID: "owner", Role: auth.RoleUser}, nil, 200, 1},
			{"other user", &auth.Claims{UserID: "other", Role: auth.RoleUser}, nil, 403, 0},
			{"admin", &auth.Claims{UserID: "admin", Role: auth.RoleAdmin}, nil, 200, 1},
			{"missing order", &auth.Claims{UserID: "owner", Role: auth.RoleUser}, apperrors.ErrNotFound, 404, 0},
			{"order failure", &auth.Claims{UserID: "owner", Role: auth.RoleUser}, context.Canceled, 500, 0},
		} {
			t.Run(adapter+"/"+tc.name, func(t *testing.T) {
				service := &handlerOrder{orderErr: tc.orderErr}
				var port Port = service
				if adapter == "remote" {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Header.Get("X-Internal-Token") != "test-token" || r.Header.Get("X-User-ID") != tc.claims.UserID || r.Header.Get("X-User-Role") != string(tc.claims.Role) {
							t.Error("remote adapter did not forward authenticated identity and service token")
						}
						switch r.URL.Path {
						case "/internal/v1/orders/order-1":
							item, err := service.GetOrder(r.Context(), "order-1")
							if err != nil {
								if appErr, ok := err.(*apperrors.AppError); ok {
									apperrors.WriteError(w, appErr)
								} else {
									apperrors.WriteError(w, apperrors.ErrInternalServer)
								}
								return
							}
							response.OK(w, OrderResponse{Order: item})
						case "/internal/v1/orders/order-1/saga":
							state, _ := service.GetSagaState(r.Context(), "order-1")
							response.OK(w, state)
						default:
							http.NotFound(w, r)
						}
					}))
					defer server.Close()
					port = NewClient(server.URL, "test-token")
				}
				request := httptest.NewRequest("GET", "/order-1/saga", nil)
				if tc.claims != nil {
					request = request.WithContext(auth.ContextWithUser(request.Context(), tc.claims))
				}
				recorder := httptest.NewRecorder()
				NewHandler(port).Routes().ServeHTTP(recorder, request)
				if recorder.Code != tc.status || service.sagaCalls != tc.sagaCalls {
					t.Fatalf("status=%d sagaCalls=%d; want status=%d sagaCalls=%d", recorder.Code, service.sagaCalls, tc.status, tc.sagaCalls)
				}
			})
		}
	}
}
