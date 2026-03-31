package service

import (
	"MrFood/services/payment/internal/repository"
	models "MrFood/services/payment/pkg"
	"context"
	"errors"
	"math/rand"
	"time"
)

var (
	ErrInvalidAmmount          = errors.New("ammount cannot be negative")
	ErrNullIdempotencyKey      = errors.New("idempotency key cannot be null")
	ErrDuplicatePaymentRequest = errors.New("duplicate payment request")
)

type paymentRepository interface {
	CreateReceipt(ctx context.Context, payment_request *models.PaymentRequest) (int32, error)
	GetReceiptByID(ctx context.Context, receipt_id, user_id int32) (*models.Receipt, error)
	GetReceiptsByUser(ctx context.Context, user_id int32) ([]*models.Receipt, error)
}

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateReceipt(ctx context.Context, payment_request *models.PaymentRequest) (int32, error) {
	if payment_request.Ammount < 0 {
		return 0, ErrInvalidAmmount
	}

	if payment_request.IdempotencyKey == "" {
		return 0, ErrNullIdempotencyKey
	}

	// mock 3rd party payment service
	time.Sleep(500 * time.Millisecond)

	success := 0.3 > rand.Float32()

	if success {
		payment_request.PaymentStatus = "success"
	} else {
		payment_request.PaymentStatus = "failed"
	}

	return s.repo.CreateReceipt(ctx, payment_request)
}

func (s *Service) GetReceiptByID(ctx context.Context, receipt_id, user_id int32) (*models.Receipt, error) {
	return s.repo.GetReceiptByID(ctx, receipt_id, user_id)
}

func (s *Service) GetReceiptsByUser(ctx context.Context, user_id int32) ([]*models.Receipt, error) {
	return s.repo.GetReceiptsByUser(ctx, user_id)
}
