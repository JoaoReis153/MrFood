package service

import (
	"MrFood/services/payment/internal/api/grpc/pb"
	"MrFood/services/payment/internal/repository"
	models "MrFood/services/payment/pkg"
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrInvalidAmmount          = errors.New("ammount cannot be negative")
	ErrNullIdempotencyKey      = errors.New("idempotency key cannot be null")
	ErrReceiptsNotFound        = errors.New("receipt not found")
	ErrDuplicatePaymentRequest = errors.New("duplicate payment request")
	ErrDatabaseNotSet          = errors.New("database is not configured")
	ErrUnauthorized            = errors.New("unauthorized access to receipt")
)

type paymentRepository interface {
	CreateReceipt(ctx context.Context, payment_request *models.Receipt) (int32, error)
	GetReceiptById(ctx context.Context, receipt_id int32, user_id int64) (*models.Receipt, error)
	GetReceiptsByUser(ctx context.Context, user_id int64) ([]*models.Receipt, error)
}

type Service struct {
	repo   paymentRepository
	client pb.PaymentToNotificationServiceClient
}

func New(repo *repository.Repository, client pb.PaymentToNotificationServiceClient) *Service {
	return &Service{repo: repo, client: client}
}

func (s *Service) CreateReceipt(ctx context.Context, receipt_request *models.Receipt) (int32, error) {
	if receipt_request.Amount < 0 {
		return 0, ErrInvalidAmmount
	}

	if receipt_request.IdempotencyKey == "" {
		return 0, ErrNullIdempotencyKey
	}

	// mock 3rd party payment service
	time.Sleep(500 * time.Millisecond)

	success := 0.3 > rand.Float32()

	if success {
		receipt_request.PaymentStatus = "success"
	} else {
		receipt_request.PaymentStatus = "failed"
	}

	if strings.Contains(receipt_request.IdempotencyKey, "BOOKING") {
		receipt_request.PaymentType = "B"
	} else {
		receipt_request.PaymentType = "S"
	}

	receipt_request.CreatedAt = time.Now().UTC()

	return s.repo.CreateReceipt(ctx, receipt_request)
}

func (s *Service) GetReceiptByID(ctx context.Context, receipt_id int32, user_id int64) error {
	receipt, err := s.repo.GetReceiptById(ctx, receipt_id, user_id)
	if err != nil {
		return err
	}

	var receipts []*models.Receipt
	receipts = append(receipts, receipt)

	_, err = s.sendReceipts(ctx, receipts)
	if err != nil {
		slog.Error("error sending receipts", "error", err)
		return err
	}

	return nil
}

func (s *Service) GetReceiptsByUser(ctx context.Context, user_id int64) error {
	receipts, err := s.repo.GetReceiptsByUser(ctx, user_id)
	if err != nil {
		return err
	}

	_, err = s.sendReceipts(ctx, receipts)
	if err != nil {
		slog.Error("error sending receipts", "error", err)
		return err
	}

	return nil
}

func (s *Service) sendReceipts(ctx context.Context, receipts []*models.Receipt) (*pb.SendReceiptsResponse, error) {
	pbReceipts := make([]*pb.Receipt, 0, len(receipts))

	for _, r := range receipts {
		pbReceipts = append(pbReceipts, &pb.Receipt{
			Amount:               float64(r.Amount),
			PaymentDescription:   r.PaymentDescription,
			CurrentPaymentStatus: r.PaymentStatus,
			PaymentType:          r.PaymentType,
			CreatedAt:            timestamppb.New(r.CreatedAt),
		})
	}

	res, err := s.client.SendReceipt(ctx, &pb.SendReceiptsRequest{
		UserEmail: receipts[0].UserEmail,
		Receipts:  pbReceipts,
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}
