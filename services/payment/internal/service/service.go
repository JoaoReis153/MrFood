package service

import (
	"MrFood/services/payment/internal/api/grpc/pb"
	"MrFood/services/payment/internal/repository"
	models "MrFood/services/payment/pkg"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrInvalidAmmount          = errors.New("ammount cannot be negative")
	ErrNullIdempotencyKey      = errors.New("idempotency key cannot be null")
	ErrReceiptNotFound         = errors.New("receipt not found")
	ErrDuplicatePaymentRequest = errors.New("duplicate payment request")
	ErrDatabaseNotSet          = errors.New("database is not configured")
	ErrUnauthorized            = errors.New("unauthorized access to receipt")
)

type paymentRepository interface {
	CreateReceipt(ctx context.Context, payment_request *models.Receipt, requestHash string) (int32, error)
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

	receipt_request.PaymentStatus = "success"
	receipt_request.CreatedAt = time.Now().UTC()

	hash, err := generateRequestHash(receipt_request)
	if err != nil {
		return 0, err
	}

	return s.repo.CreateReceipt(ctx, receipt_request, hash)
}

func (s *Service) GetReceiptById(ctx context.Context, receipt_id int32, user_id int64) error {
	receipt, err := s.repo.GetReceiptById(ctx, receipt_id, user_id)
	if err != nil {
		return err
	}

	var receipts []*models.Receipt
	receipts = append(receipts, receipt)

	if _, err = s.sendReceipts(ctx, receipts); err != nil {
		// Email delivery is best-effort; log but don't fail the request.
		slog.Warn("receipt email delivery failed, continuing", "error", err)
	}

	return nil
}

func (s *Service) GetReceiptsByUser(ctx context.Context, user_id int64) error {
	receipts, err := s.repo.GetReceiptsByUser(ctx, user_id)
	if err != nil {
		return err
	}

	if _, err = s.sendReceipts(ctx, receipts); err != nil {
		// Email delivery is best-effort; log but don't fail the request.
		slog.Warn("receipt email delivery failed, continuing", "error", err)
	}

	return nil
}

func (s *Service) sendReceipts(ctx context.Context, receipts []*models.Receipt) (*pb.SendReceiptsResponse, error) {
	if len(receipts) == 0 {
		return nil, ErrReceiptNotFound
	}

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

	res, err := s.client.SendReceipts(ctx, &pb.SendReceiptsRequest{
		UserEmail: receipts[0].UserEmail,
		Receipts:  pbReceipts,
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}

func generateRequestHash(r *models.Receipt) (string, error) {
	input := struct {
		UserID      int64
		Amount      float32
		Description string
		PaymentType string
		TimeSlot    string
	}{
		UserID:      r.UserID,
		Amount:      r.Amount,
		Description: r.PaymentDescription,
		PaymentType: r.PaymentType,
		TimeSlot:    r.CreatedAt.UTC().Format("2006-01-02T15:04"),
	}

	data, err := json.Marshal(input)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
