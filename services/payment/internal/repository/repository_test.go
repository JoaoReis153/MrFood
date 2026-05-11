package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	models "MrFood/services/payment/pkg"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
)

// -----------------------------
// CreateReceipt Tests
// -----------------------------
func TestCreateReceipt(t *testing.T) {
	ctx := context.Background()

	baseReceipt := &models.Receipt{
		IdempotencyKey:     "key",
		UserID:             1,
		UserEmail:          "test@test.com",
		Amount:             10,
		PaymentDescription: "order",
		PaymentStatus:      "success",
		PaymentType:        "card",
		CreatedAt:          time.Now(),
	}

	t.Run("successful insert", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo, _ := New(ctx, nil, mock)

		mock.ExpectQuery(`INSERT INTO receipts`).
			WithArgs(
				baseReceipt.IdempotencyKey,
				"hash",
				baseReceipt.UserID,
				baseReceipt.UserEmail,
				baseReceipt.Amount,
				baseReceipt.PaymentDescription,
				baseReceipt.PaymentStatus,
				baseReceipt.PaymentType,
				baseReceipt.CreatedAt,
			).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(42)))

		id, err := repo.CreateReceipt(ctx, baseReceipt, "hash")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != 42 {
			t.Fatalf("expected id 42, got %d", id)
		}
	})

	t.Run("duplicate same hash returns existing id", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo, _ := New(ctx, nil, mock)

		mock.ExpectQuery(`INSERT INTO receipts`).
			WithArgs(
				baseReceipt.IdempotencyKey,
				"hash",
				baseReceipt.UserID,
				baseReceipt.UserEmail,
				baseReceipt.Amount,
				baseReceipt.PaymentDescription,
				baseReceipt.PaymentStatus,
				baseReceipt.PaymentType,
				baseReceipt.CreatedAt,
			).
			WillReturnError(pgx.ErrNoRows)

		mock.ExpectQuery(`SELECT id, request_hash FROM receipts`).
			WithArgs(baseReceipt.IdempotencyKey).
			WillReturnRows(
				pgxmock.NewRows([]string{"id", "request_hash"}).
					AddRow(int32(99), "hash"),
			)

		id, err := repo.CreateReceipt(ctx, baseReceipt, "hash")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != 99 {
			t.Fatalf("expected id 99, got %d", id)
		}
	})

	t.Run("duplicate different hash", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo, _ := New(ctx, nil, mock)

		mock.ExpectQuery(`INSERT INTO receipts`).
			WithArgs(
				baseReceipt.IdempotencyKey,
				"hash",
				baseReceipt.UserID,
				baseReceipt.UserEmail,
				baseReceipt.Amount,
				baseReceipt.PaymentDescription,
				baseReceipt.PaymentStatus,
				baseReceipt.PaymentType,
				baseReceipt.CreatedAt,
			).
			WillReturnError(pgx.ErrNoRows)

		mock.ExpectQuery(`SELECT id, request_hash FROM receipts`).
			WithArgs(baseReceipt.IdempotencyKey).
			WillReturnRows(
				pgxmock.NewRows([]string{"id", "request_hash"}).
					AddRow(int32(99), "different_hash"),
			)

		_, err := repo.CreateReceipt(ctx, baseReceipt, "hash")
		if !errors.Is(err, ErrDuplicatePaymentRequest) {
			t.Fatalf("expected ErrDuplicatePaymentRequest, got %v", err)
		}
	})

	t.Run("error checking existing hash", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo, _ := New(ctx, nil, mock)

		mock.ExpectQuery(`INSERT INTO receipts`).
			WillReturnError(pgx.ErrNoRows)

		mock.ExpectQuery(`SELECT id, request_hash FROM receipts`).
			WillReturnError(errors.New("db error"))

		_, err := repo.CreateReceipt(ctx, baseReceipt, "hash")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("insert unexpected error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo, _ := New(ctx, nil, mock)

		mock.ExpectQuery(`INSERT INTO receipts`).
			WillReturnError(errors.New("insert failed"))

		_, err := repo.CreateReceipt(ctx, baseReceipt, "hash")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

// -----------------------------
// GetReceiptById Tests
// -----------------------------
func TestGetReceiptById(t *testing.T) {
	ctx := context.Background()

	t.Run("database not set", func(t *testing.T) {
		repo := &Repository{DB: nil}

		_, err := repo.GetReceiptById(ctx, 1, 1)
		if !errors.Is(err, ErrDatabaseNotSet) {
			t.Fatalf("expected ErrDatabaseNotSet, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo, _ := New(ctx, nil, mock)

		mock.ExpectQuery(`SELECT`).
			WithArgs(int32(1), int64(1)).
			WillReturnError(pgx.ErrNoRows)

		_, err := repo.GetReceiptById(ctx, 1, 1)
		if !errors.Is(err, ErrReceiptNotFound) {
			t.Fatalf("expected ErrReceiptNotFound, got %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo, _ := New(ctx, nil, mock)

		mock.ExpectQuery(`SELECT`).
			WillReturnError(errors.New("db error"))

		_, err := repo.GetReceiptById(ctx, 1, 1)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo, _ := New(ctx, nil, mock)

		mock.ExpectQuery(`SELECT`).
			WithArgs(int32(1), int64(1)).
			WillReturnRows(
				pgxmock.NewRows([]string{
					"id", "idempotency_key", "request_hash",
					"user_id", "user_email", "amount",
					"payment_description", "current_payment_status",
					"payment_type", "created_at",
				}).AddRow(
					int32(1), "key", "hash", int64(1),
					"test@test.com", float32(10),
					"order", "success", "card", time.Now(),
				),
			)

		receipt, err := repo.GetReceiptById(ctx, 1, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if receipt.ID != 1 {
			t.Fatalf("expected id 1, got %d", receipt.ID)
		}
	})
}

// -----------------------------
// GetReceiptsByUser Tests
// -----------------------------
func TestGetReceiptsByUser(t *testing.T) {
	ctx := context.Background()

	t.Run("database not set", func(t *testing.T) {
		repo := &Repository{DB: nil}

		_, err := repo.GetReceiptsByUser(ctx, 1)
		if !errors.Is(err, ErrDatabaseNotSet) {
			t.Fatalf("expected ErrDatabaseNotSet, got %v", err)
		}
	})

	t.Run("query error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo, _ := New(ctx, nil, mock)

		mock.ExpectQuery(`SELECT`).
			WithArgs(int64(1)).
			WillReturnError(errors.New("query failed"))

		_, err := repo.GetReceiptsByUser(ctx, 1)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo, _ := New(ctx, nil, mock)

		rows := pgxmock.NewRows([]string{"id"}).AddRow("invalid") // wrong type

		mock.ExpectQuery(`SELECT`).
			WithArgs(int64(1)).
			WillReturnRows(rows)

		_, err := repo.GetReceiptsByUser(ctx, 1)
		if err == nil {
			t.Fatalf("expected scan error")
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo, _ := New(ctx, nil, mock)

		mock.ExpectQuery(`SELECT`).
			WithArgs(int64(1)).
			WillReturnRows(
				pgxmock.NewRows([]string{
					"id", "idempotency_key", "request_hash",
					"user_id", "user_email", "amount",
					"payment_description", "current_payment_status",
					"payment_type", "created_at",
				}).AddRow(
					int32(1), "key", "hash", int64(1),
					"test@test.com", float32(10),
					"order", "success", "card", time.Now(),
				),
			)

		receipts, err := repo.GetReceiptsByUser(ctx, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(receipts) != 1 {
			t.Fatalf("expected 1 receipt, got %d", len(receipts))
		}
	})
}
