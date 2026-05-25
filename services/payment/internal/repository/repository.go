package repository

import (
	"MrFood/services/payment/config"
	models "MrFood/services/payment/pkg"
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrReceiptNotFound         = errors.New("receipt not found")
	ErrDatabaseNotSet          = errors.New("database is not configured")
	ErrUnauthorized            = errors.New("unauthorized access to receipt")
	ErrDuplicatePaymentRequest = errors.New("duplicate payment request")
)

type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Repository struct {
	DB DB
}

func New(_ context.Context, _ *config.Config, db DB) (*Repository, error) {
	return &Repository{DB: db}, nil
}

func (r *Repository) Close(_ context.Context) error {
	return nil
}

func (r *Repository) CreateReceipt(ctx context.Context, receipt *models.Receipt, requestHash string) (int32, error) {
	query := `
		INSERT INTO receipts (
		idempotency_key, request_hash, user_id, user_email,
		amount, payment_description, current_payment_status,
		payment_type, payment_intent_id, created_at
	)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	ON CONFLICT (idempotency_key) DO NOTHING
	RETURNING id;
	`
	var receipt_id int32

	err := r.DB.QueryRow(ctx, query,
		receipt.IdempotencyKey,
		requestHash,
		receipt.UserID,
		receipt.UserEmail,
		receipt.Amount,
		receipt.PaymentDescription,
		receipt.PaymentStatus,
		receipt.PaymentType,
		receipt.PaymentIntentID,
		receipt.CreatedAt,
	).Scan(&receipt_id)

	if err == nil {
		return receipt_id, nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		existing := `
			SELECT id, request_hash
			FROM receipts
			WHERE idempotency_key = $1;
		`

		var existingID int32
		var existingHash string

		err = r.DB.QueryRow(ctx, existing, receipt.IdempotencyKey).
			Scan(&existingID, &existingHash)
		if err != nil {
			slog.ErrorContext(ctx, "error checking hash", "error", err)
			return 0, err
		}

		if existingHash != requestHash {
			slog.ErrorContext(ctx, "duplicate payment")
			return 0, ErrDuplicatePaymentRequest
		}

		return existingID, nil
	}

	return 0, err
}

func (r *Repository) UpdateReceiptStatus(ctx context.Context, paymentIntentID string, status string) error {
	if r.DB == nil {
		return ErrDatabaseNotSet
	}

	query := `
		UPDATE receipts
		SET current_payment_status = $1
		WHERE payment_intent_id = $2
	`

	result, err := r.DB.Exec(ctx, query, status, paymentIntentID)
	if err != nil {
		slog.Error("failed to update receipt status", "payment_intent_id", paymentIntentID, "error", err)
		return err
	}

	if result.RowsAffected() == 0 {
		slog.Warn("no receipt found for payment_intent_id", "payment_intent_id", paymentIntentID)
		return ErrReceiptNotFound
	}

	return nil
}

func (r *Repository) GetReceiptByPaymentIntentID(ctx context.Context, paymentIntentID string) (*models.Receipt, error) {
	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	query := `
		SELECT
			id, idempotency_key, request_hash, user_id, user_email,
			amount, payment_description, current_payment_status,
			payment_type, payment_intent_id, created_at
		FROM receipts
		WHERE payment_intent_id = $1
	`

	receipt := &models.Receipt{}

	err := r.DB.QueryRow(ctx, query, paymentIntentID).Scan(
		&receipt.ID,
		&receipt.IdempotencyKey,
		&receipt.RequestHash,
		&receipt.UserID,
		&receipt.UserEmail,
		&receipt.Amount,
		&receipt.PaymentDescription,
		&receipt.PaymentStatus,
		&receipt.PaymentType,
		&receipt.PaymentIntentID,
		&receipt.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReceiptNotFound
		}
		return nil, err
	}

	return receipt, nil
}

func (r *Repository) GetReceiptById(ctx context.Context, receipt_id int32, user_id int64) (*models.Receipt, error) {
	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	query := `
		SELECT
			id,
			idempotency_key,
			request_hash,
			user_id,
			user_email,
			amount,
			payment_description,
			current_payment_status,
			payment_type,
			payment_intent_id,
			created_at
		FROM receipts
		WHERE id = $1
		AND user_id = $2
	`

	receipt := &models.Receipt{}

	err := r.DB.QueryRow(ctx, query, receipt_id, user_id).Scan(
		&receipt.ID,
		&receipt.IdempotencyKey,
		&receipt.RequestHash,
		&receipt.UserID,
		&receipt.UserEmail,
		&receipt.Amount,
		&receipt.PaymentDescription,
		&receipt.PaymentStatus,
		&receipt.PaymentType,
		&receipt.PaymentIntentID,
		&receipt.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReceiptNotFound
		}
		return nil, err
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
		slog.ErrorContext(ctx, "error querying DB", "error", err)
		return nil, err
	}
	defer receiptsRows.Close()

	var receipts []*models.Receipt

	for receiptsRows.Next() {
		curr := &models.Receipt{}

		err := receiptsRows.Scan(
			&curr.ID,
			&curr.IdempotencyKey,
			&curr.RequestHash,
			&curr.UserID,
			&curr.UserEmail,
			&curr.Amount,
			&curr.PaymentDescription,
			&curr.PaymentStatus,
			&curr.PaymentType,
			&curr.PaymentIntentID,
			&curr.CreatedAt,
		)
		if err != nil {
			slog.ErrorContext(ctx, "error scanning receipt row", "error", err)
			return nil, err
		}

		receipts = append(receipts, curr)
	}
	if err := receiptsRows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating receipts", "error", err)
		return nil, err
	}

	return receipts, nil
}
