package saga

import (
	"context"
	"errors"
	"testing"
)

type fakeOrderService struct {
	confirmed bool
	failed    bool
}

func (f *fakeOrderService) ConfirmOrder(ctx context.Context, orderID string) error {
	f.confirmed = true
	return nil
}

func (f *fakeOrderService) CancelOrder(ctx context.Context, orderID string) error {
	return nil
}

func (f *fakeOrderService) FailOrder(ctx context.Context, orderID string) error {
	f.failed = true
	return nil
}

type fakeInventoryService struct {
	reserved  bool
	released  bool
	committed bool
}

func (f *fakeInventoryService) ReserveStock(ctx context.Context, orderID string, items []OrderItem) error {
	f.reserved = true
	return nil
}

func (f *fakeInventoryService) ReleaseStock(ctx context.Context, orderID string) error {
	f.released = true
	return nil
}

func (f *fakeInventoryService) CommitStock(ctx context.Context, orderID string) error {
	f.committed = true
	return nil
}

type fakePaymentService struct {
	err error
}

func (f *fakePaymentService) ProcessPayment(ctx context.Context, orderID, userID string, amount float64, currency, method, idempotencyKey string) error {
	return f.err
}

func TestOrderPlacementSagaHappyPath(t *testing.T) {
	orderSvc := &fakeOrderService{}
	inventorySvc := &fakeInventoryService{}
	paymentSvc := &fakePaymentService{}

	sg := OrderPlacementSaga(orderSvc, inventorySvc, paymentSvc)
	err := NewOrchestrator().Execute(context.Background(), sg, OrderPlacementData{
		OrderID:        "order-1",
		UserID:         "user-1",
		Items:          []OrderItem{{SKU: "LAPTOP-001", Quantity: 1}},
		TotalAmount:    99.99,
		Currency:       "USD",
		PaymentMethod:  "demo_card",
		IdempotencyKey: "SAGA-order-1",
	})

	if err != nil {
		t.Fatalf("expected happy path to succeed, got %v", err)
	}
	if !inventorySvc.reserved || !inventorySvc.committed || !orderSvc.confirmed {
		t.Fatalf("expected reserve, commit, and confirm; got reserved=%v committed=%v confirmed=%v",
			inventorySvc.reserved, inventorySvc.committed, orderSvc.confirmed)
	}
	if inventorySvc.released || orderSvc.failed {
		t.Fatalf("did not expect compensation on happy path")
	}
}

func TestOrderPlacementSagaPaymentFailureCompensates(t *testing.T) {
	orderSvc := &fakeOrderService{}
	inventorySvc := &fakeInventoryService{}
	paymentSvc := &fakePaymentService{err: errors.New("payment declined")}

	sg := OrderPlacementSaga(orderSvc, inventorySvc, paymentSvc)
	err := NewOrchestrator().Execute(context.Background(), sg, OrderPlacementData{
		OrderID:        "order-2",
		UserID:         "user-1",
		Items:          []OrderItem{{SKU: "FAIL-PAYMENT-001", Quantity: 1}},
		TotalAmount:    19.99,
		Currency:       "USD",
		PaymentMethod:  "demo_card",
		IdempotencyKey: "FAIL-order-2",
	})

	if !errors.Is(err, ErrCompensated) {
		t.Fatalf("expected compensated saga error, got %v", err)
	}
	if !inventorySvc.reserved || !inventorySvc.released || !orderSvc.failed {
		t.Fatalf("expected reserve, release, and failed order; got reserved=%v released=%v failed=%v",
			inventorySvc.reserved, inventorySvc.released, orderSvc.failed)
	}
	if inventorySvc.committed || orderSvc.confirmed {
		t.Fatalf("did not expect commit or confirm after payment failure")
	}
}
