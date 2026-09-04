package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLiveHandlerIsIndependentOfDependencies(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	res := httptest.NewRecorder()

	LiveHandler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	assertStatus(t, res, "alive")
}

func TestReadyHandlerReturnsReadyWhenAllChecksPass(t *testing.T) {
	called := false
	check := func(ctx context.Context) error {
		called = true
		return nil
	}
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	res := httptest.NewRecorder()

	ReadyHandler(check).ServeHTTP(res, req)

	if !called {
		t.Fatal("readiness check was not called")
	}
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	assertStatus(t, res, "ready")
}

func TestReadyHandlerReturnsUnavailableWhenCheckFails(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	res := httptest.NewRecorder()

	ReadyHandler(func(context.Context) error { return errors.New("database unavailable") }).ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
	assertStatus(t, res, "not_ready")
}

func TestReadyHandlerStopsAfterFirstFailedCheck(t *testing.T) {
	secondCalled := false
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	res := httptest.NewRecorder()

	ReadyHandler(
		func(context.Context) error { return errors.New("database unavailable") },
		func(context.Context) error {
			secondCalled = true
			return nil
		},
	).ServeHTTP(res, req)

	if secondCalled {
		t.Fatal("readiness handler evaluated checks after the first failure")
	}
}

func assertStatus(t *testing.T, res *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != want {
		t.Fatalf("status body = %q, want %q", body.Status, want)
	}
}
