package repository

import (
	"MrFood/services/payment/config"
	models "MrFood/services/payment/pkg"
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrReceiptNotFound         = errors.New("receipt not found")
	ErrDatabaseNotSet          = errors.New("database is not configured")
	ErrUnauthorized            = errors.New("unauthorized access to receipt")
	ErrDuplicatePaymentRequest = errors.New("duplicate payment request")
)

type Repository struct {
	DB *pgxpool.Pool
}

func New(_ context.Context, _ *config.Config, db *pgxpool.Pool) (*Repository, error) {
	return &Repository{
		DB: db,
	}, nil
}

func (r *Repository) Close(_ context.Context) error {
	return nil
}

func (r *Repository) CreateReceipt(ctx context.Context, receipt_request *models.Receipt) (int32, error) {
	if r.DB == nil {
		return 0, ErrDatabaseNotSet
	}

	query := `
		INSERT INTO receipts (idempotency_key, user_id, user_email, ammount, payment_description, current_payment_status, 
				payment_type, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id;
	`

	var receipt_id int32

	err := r.DB.QueryRow(ctx, query, receipt_request.IdempotencyKey, receipt_request.UserID, receipt_request.UserEmail,
		receipt_request.Amount, receipt_request.PaymentDescription, receipt_request.PaymentStatus, receipt_request.PaymentType,
		receipt_request.CreatedAt).Scan(&receipt_id)

	var pgErr *pgconn.PgError

	if err != nil {
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				return receipt_id, nil
			}
		}
		return 0, err
	}

	return receipt_id, nil
}

func (r *Repository) GetReceiptById(ctx context.Context, receipt_id int32, user_id int64) (*models.Receipt, error) {
	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	query := `
		SELECT *
		FROM receipts
		WHERE id = $1
		AND user_id = $2
	`

	receipt := &models.Receipt{}

	err := r.DB.QueryRow(ctx, query, receipt_id, user_id).Scan(
		&receipt.ID,
		&receipt.IdempotencyKey,
		&receipt.UserID,
		&receipt.UserEmail,
		&receipt.Amount,
		&receipt.PaymentDescription,
		&receipt.PaymentStatus,
		&receipt.PaymentType,
		&receipt.CreatedAt,
	)

	if err != nil {
		slog.Error("receipt not found", "error", err)
		return nil, ErrReceiptNotFound
	}

	return receipt, nil
}

func (r *Repository) GetReceiptsByUser(ctx context.Context, user_id int64) ([]*models.Receipt, error) {
	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	query := `
		SELECT *
		FROM receipts
		WHERE user_id = $1
	`

	receiptsRows, err := r.DB.Query(ctx, query, user_id)

	if err != nil {
		slog.Error("error querying DB", "error", err)
		return nil, err
	}
	defer receiptsRows.Close()

	var receipts []*models.Receipt

	for receiptsRows.Next() {
		var curr_receipt *models.Receipt
		if err := receiptsRows.Scan(&curr_receipt); err != nil {
			slog.Error("error scanning receipt row", "error", err)
			return nil, err
		}
		receipts = append(receipts, curr_receipt)
	}
	if receiptsRows.Err() != nil {
		slog.Error("error iterating receipts", "error", err)
		return nil, err
	}

	return receipts, nil
}
