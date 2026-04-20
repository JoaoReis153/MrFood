package service

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "MrFood/services/payment/internal/api/grpc/pb"
	models "MrFood/services/payment/pkg"

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
		service := &Service{
			repo:   &mockRepo{},
			client: &mockNotificationClient{err: errors.New("grpc fail")},
		}

		err := service.GetReceiptById(context.Background(), 1, 1)
		if err == nil {
			t.Fatal("expected error")
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
		service := &Service{
			repo:   &mockRepo{},
			client: &mockNotificationClient{err: errors.New("grpc fail")},
		}

		err := service.GetReceiptsByUser(context.Background(), 1)
		if err == nil {
			t.Fatal("expected error")
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

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty slice")
		}
	}()

	_, _ = service.sendReceipts(context.Background(), []*models.Receipt{})
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
