package app

import (
	"context"
	"testing"

	pb "MrFood/services/payment/internal/api/grpc/pb"
	"MrFood/services/payment/internal/service"
	models "MrFood/services/payment/pkg"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/metadata"
)

// -----------------------------
// Mock Services
// -----------------------------
type mockCommandService struct{}

func (m *mockCommandService) CreateReceipt(ctx context.Context, r *models.Receipt) (int32, error) {
	if r.Amount <= 0 {
		return 0, service.ErrInvalidAmmount
	}
	if r.IdempotencyKey == "" {
		return 0, service.ErrNullIdempotencyKey
	}
	if r.IdempotencyKey == "duplicate" {
		return 0, service.ErrDuplicatePaymentRequest
	}
	return 100, nil
}

type mockQueryService struct{}

func (m *mockQueryService) GetReceiptsByUser(ctx context.Context, userID int64) error {
	if userID == uuidToInt64("0") {
		return service.ErrUnauthorized
	}
	return nil
}

func (m *mockQueryService) GetReceiptById(ctx context.Context, receiptID int32, userID int64) error {
	if receiptID == 0 {
		return service.ErrReceiptNotFound
	}
	if userID == uuidToInt64("0") {
		return service.ErrUnauthorized
	}
	return nil
}

// -----------------------------
// Helpers
// -----------------------------
func contextWithAuth(userID string) context.Context {
	claims := &Claims{UserID: userID}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("secret"))

	md := metadata.New(map[string]string{
		"authorization": "Bearer " + tokenStr,
	})

	return metadata.NewIncomingContext(context.Background(), md)
}

// -----------------------------
// MakePayment Tests
// -----------------------------
func TestMakePayment_EdgeCases(t *testing.T) {
	server := &commandServer{
		paymentService: &mockCommandService{},
	}

	tests := []struct {
		name      string
		req       *pb.PaymentRequest
		expectErr bool
	}{
		{
			name: "Invalid amount",
			req: &pb.PaymentRequest{
				Amount:         0,
				IdempotencyKey: "key",
				UserEmail:      "test@test.com",
				UserId:         1,
			},
			expectErr: true,
		},
		{
			name: "Missing idempotency key",
			req: &pb.PaymentRequest{
				Amount: 10,
				UserId: 1,
			},
			expectErr: true,
		},
		{
			name: "Duplicate request",
			req: &pb.PaymentRequest{
				Amount:         10,
				IdempotencyKey: "duplicate",
				UserId:         1,
			},
			expectErr: true,
		},
		{
			name: "Successful payment",
			req: &pb.PaymentRequest{
				Amount:         10,
				IdempotencyKey: "valid",
				UserEmail:      "test@test.com",
				UserId:         1,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := server.MakePayment(context.Background(), tt.req)

			if tt.expectErr && err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !tt.expectErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if res.ReceiptId != 100 {
					t.Fatalf("expected receipt id 100, got %d", res.ReceiptId)
				}
			}
		})
	}
}

// -----------------------------
// GetReceiptsByUser Tests
// -----------------------------
func TestGetReceiptsByUser_EdgeCases(t *testing.T) {
	server := &queryServer{
		paymentService: &mockQueryService{},
	}

	tests := []struct {
		name        string
		ctx         context.Context
		expectError bool
	}{
		{
			name:        "Missing metadata",
			ctx:         context.Background(),
			expectError: true,
		},
		{
			name:        "Invalid user id in token",
			ctx:         contextWithAuth("0"),
			expectError: true,
		},
		{
			name:        "Successful request",
			ctx:         contextWithAuth("1"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.GetReceiptsByUser(tt.ctx, &pb.ReceiptRequest{})

			if tt.expectError && err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// -----------------------------
// GetReceiptById Tests
// -----------------------------
func TestGetReceiptById_EdgeCases(t *testing.T) {
	server := &queryServer{
		paymentService: &mockQueryService{},
	}

	tests := []struct {
		name        string
		ctx         context.Context
		req         *pb.ReceiptRequest
		expectError bool
	}{
		{
			name:        "Missing metadata",
			ctx:         context.Background(),
			req:         &pb.ReceiptRequest{ReceiptId: 1},
			expectError: true,
		},
		{
			name:        "Receipt not found",
			ctx:         contextWithAuth("1"),
			req:         &pb.ReceiptRequest{ReceiptId: 0},
			expectError: true,
		},
		{
			name:        "Unauthorized user",
			ctx:         contextWithAuth("0"),
			req:         &pb.ReceiptRequest{ReceiptId: 1},
			expectError: true,
		},
		{
			name:        "Successful request",
			ctx:         contextWithAuth("1"),
			req:         &pb.ReceiptRequest{ReceiptId: 1},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.GetReceiptById(tt.ctx, tt.req)

			if tt.expectError && err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// -----------------------------
// ExtractUserFromContext Tests
// -----------------------------
func TestExtractUserFromContext(t *testing.T) {
	t.Run("No metadata", func(t *testing.T) {
		_, err := ExtractUserFromContext(context.Background())
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("No auth header", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
		_, err := ExtractUserFromContext(ctx)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("Invalid token", func(t *testing.T) {
		md := metadata.New(map[string]string{
			"authorization": "Bearer invalid.token.here",
		})
		ctx := metadata.NewIncomingContext(context.Background(), md)

		_, err := ExtractUserFromContext(ctx)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("Valid token", func(t *testing.T) {
		ctx := contextWithAuth("1")
		claims, err := ExtractUserFromContext(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if claims.UserID != "1" {
			t.Fatalf("expected user id 1, got %s", claims.UserID)
		}
	})
}
