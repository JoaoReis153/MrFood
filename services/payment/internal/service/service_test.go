package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"MrFood/services/payment/config"
	pb "MrFood/services/payment/internal/api/grpc/pb"
	models "MrFood/services/payment/pkg"

	"github.com/stripe/stripe-go/v85"
	"google.golang.org/grpc"
)

// -----------------------------
// Mock Repo
// -----------------------------
type mockRepo struct {
	createErr error
	getErr    error
	listErr   error
}

func (m *mockRepo) CreateReceipt(ctx context.Context, r *models.Receipt, hash string) (int32, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	return 42, nil
}

func (m *mockRepo) UpdateReceiptStatus(ctx context.Context, paymentIntentID string, status string) error {
	return nil
}

func (m *mockRepo) GetReceiptById(ctx context.Context, receiptID int32, userID int64) (*models.Receipt, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}

	return &models.Receipt{
		UserID:             userID,
		UserEmail:          "test@test.com",
		Amount:             10,
		PaymentDescription: "order-1",
		PaymentStatus:      "success",
		PaymentType:        "card",
		CreatedAt:          time.Now(),
	}, nil
}

func (m *mockRepo) GetReceiptByPaymentIntentID(ctx context.Context, paymentIntentID string) (*models.Receipt, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}

	return &models.Receipt{
		UserID:             1,
		UserEmail:          "test@test.com",
		Amount:             10,
		PaymentDescription: "order-1",
		PaymentStatus:      "success",
		PaymentType:        "card",
		PaymentIntentID:    paymentIntentID,
		CreatedAt:          time.Now(),
	}, nil
}

func (m *mockRepo) GetReceiptsByUser(ctx context.Context, userID int64) ([]*models.Receipt, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	return []*models.Receipt{
		{
			UserID:             userID,
			UserEmail:          "test@test.com",
			Amount:             20,
			PaymentDescription: "order-2",
			PaymentStatus:      "success",
			PaymentType:        "card",
			CreatedAt:          time.Now(),
		},
	}, nil
}

// -----------------------------
// Mock gRPC Client
// -----------------------------
type mockNotificationClient struct {
	err   error
	calls int
}

func (m *mockNotificationClient) SendReceipts(
	ctx context.Context,
	req *pb.SendReceiptsRequest,
	opts ...grpc.CallOption,
) (*pb.SendReceiptsResponse, error) {
	m.calls++

	if m.err != nil {
		return nil, m.err
	}

	return &pb.SendReceiptsResponse{}, nil
}

// -----------------------------
// CreateReceipt Tests
// -----------------------------
func TestCreateReceipt_EdgeCases(t *testing.T) {
	originalCreatePaymentIntent := createPaymentIntent
	createPaymentIntent = func(params *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		return &stripe.PaymentIntent{ID: "pi_test", Status: stripe.PaymentIntentStatusSucceeded}, nil
	}
	defer func() {
		createPaymentIntent = originalCreatePaymentIntent
	}()

	service := &Service{
		repo:   &mockRepo{},
		client: &mockNotificationClient{},
	}

	tests := []struct {
		name      string
		receipt   *models.Receipt
		expectErr error
	}{
		{
			name: "Negative amount",
			receipt: &models.Receipt{
				Amount:         -1,
				IdempotencyKey: "key",
			},
			expectErr: ErrInvalidAmmount,
		},
		{
			name: "Missing idempotency key",
			receipt: &models.Receipt{
				Amount: 10,
			},
			expectErr: ErrNullIdempotencyKey,
		},
		{
			name: "Successful creation",
			receipt: &models.Receipt{
				UserID:             1,
				UserEmail:          "test@test.com",
				Amount:             10,
				IdempotencyKey:     "valid",
				PaymentDescription: "order",
				PaymentType:        "card",
			},
			expectErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := service.CreateReceipt(context.Background(), tt.receipt)

			if !errors.Is(err, tt.expectErr) {
				t.Fatalf("expected error %v, got %v", tt.expectErr, err)
			}

			if tt.expectErr == nil && id != 42 {
				t.Fatalf("expected id 42, got %d", id)
			}
		})
	}
}

// -----------------------------
// GetReceiptById Tests
// -----------------------------
func TestGetReceiptById(t *testing.T) {
	t.Run("Repo error", func(t *testing.T) {
		service := &Service{
			repo:   &mockRepo{getErr: errors.New("db fail")},
			client: &mockNotificationClient{},
		}

		err := service.GetReceiptById(context.Background(), 1, 1)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SendReceipts error", func(t *testing.T) {
		// Email delivery is best-effort: a notification failure must not
		// cause GetReceiptById to return an error.
		nc := &mockNotificationClient{err: errors.New("grpc fail")}
		service := &Service{
			repo:   &mockRepo{},
			client: nc,
		}

		err := service.GetReceiptById(context.Background(), 1, 1)
		if err != nil {
			t.Fatalf("expected nil error when email delivery fails, got: %v", err)
		}
		if nc.calls != 1 {
			t.Fatalf("expected SendReceipts to be called once, got %d", nc.calls)
		}
	})

	t.Run("Success", func(t *testing.T) {
		service := &Service{
			repo:   &mockRepo{},
			client: &mockNotificationClient{},
		}

		err := service.GetReceiptById(context.Background(), 1, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// -----------------------------
// GetReceiptsByUser Tests
// -----------------------------
func TestGetReceiptsByUser(t *testing.T) {
	t.Run("Repo error", func(t *testing.T) {
		service := &Service{
			repo:   &mockRepo{listErr: errors.New("db fail")},
			client: &mockNotificationClient{},
		}

		err := service.GetReceiptsByUser(context.Background(), 1)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SendReceipts error", func(t *testing.T) {
		// Email delivery is best-effort: a notification failure must not
		// cause GetReceiptsByUser to return an error.
		nc := &mockNotificationClient{err: errors.New("grpc fail")}
		service := &Service{
			repo:   &mockRepo{},
			client: nc,
		}

		err := service.GetReceiptsByUser(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected nil error when email delivery fails, got: %v", err)
		}
		if nc.calls != 1 {
			t.Fatalf("expected SendReceipts to be called once, got %d", nc.calls)
		}
	})

	t.Run("Success", func(t *testing.T) {
		service := &Service{
			repo:   &mockRepo{},
			client: &mockNotificationClient{},
		}

		err := service.GetReceiptsByUser(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// -----------------------------
// sendReceipts Edge Case
// -----------------------------
func TestSendReceipts_EmptySlice(t *testing.T) {
	service := &Service{
		repo:   &mockRepo{},
		client: &mockNotificationClient{},
	}

	_, err := service.sendReceipts(context.Background(), []*models.Receipt{})

	if !errors.Is(err, ErrReceiptNotFound) {
		t.Fatalf("expected ErrReceiptNotFound, got %v", err)
	}
}

// -----------------------------
// generateRequestHash Tests
// -----------------------------
func TestGenerateRequestHash(t *testing.T) {
	r1 := &models.Receipt{
		UserID:             1,
		Amount:             10,
		PaymentDescription: "order",
		PaymentType:        "card",
		CreatedAt:          time.Now().UTC(),
	}

	r2 := *r1

	hash1, _ := generateRequestHash(r1)
	hash2, _ := generateRequestHash(&r2)

	if hash1 != hash2 {
		t.Fatalf("expected equal hashes, got %s and %s", hash1, hash2)
	}
}

func TestNewUsesDemoPaymentWithoutStripe(t *testing.T) {
	cfg := &config.Config{}
	cfg.Stripe.DemoMode = true
	s := New(nil, &mockNotificationClient{}, cfg)
	if s.createIntent == nil {
		t.Fatal("expected demo payment intent func")
	}
	s.repo = &mockRepo{}

	r := &models.Receipt{Amount: 1000, IdempotencyKey: "abc", PaymentDescription: "test"}
	if _, err := s.CreateReceipt(context.Background(), r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(r.PaymentIntentID, "pi_demo_") {
		t.Fatalf("expected demo payment intent id, got %q", r.PaymentIntentID)
	}
	if r.PaymentStatus != string(stripe.PaymentIntentStatusSucceeded) {
		t.Fatalf("expected succeeded status, got %q", r.PaymentStatus)
	}
}

func TestDemoPaymentIntentIDs(t *testing.T) {
	newParams := func(key string) *stripe.PaymentIntentParams {
		p := &stripe.PaymentIntentParams{Amount: stripe.Int64(100)}
		p.SetIdempotencyKey(key)
		return p
	}
	a1, _ := demoPaymentIntent(newParams("a"))
	a2, _ := demoPaymentIntent(newParams("a"))
	b, _ := demoPaymentIntent(newParams("b"))
	if a1.ID != a2.ID {
		t.Fatalf("same idempotency key gave different ids: %s, %s", a1.ID, a2.ID)
	}
	if a1.ID == b.ID {
		t.Fatalf("different idempotency keys gave same id: %s", a1.ID)
	}
}

func TestNewUsesStripeUnlessDemoWithoutKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		demoMode bool
	}{
		{name: "key set", key: "sk_test_x"},
		{name: "key set with demo mode", key: "sk_test_x", demoMode: true},
		{name: "no key without demo mode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Stripe.SecretKey = tt.key
			cfg.Stripe.DemoMode = tt.demoMode
			s := New(nil, &mockNotificationClient{}, cfg)
			if s.createIntent != nil {
				t.Fatal("expected real Stripe client")
			}
		})
	}
}
