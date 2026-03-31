package service

import (
	"MrFood/services/payment/internal/repository"
	models "MrFood/services/payment/pkg"
	"context"
)

type paymentRepository interface {
	CreateReceipt(ctx context.Context, payment_request *models.PaymentRequest) (int32, error)
	GetReceiptByID(ctx context.Context, receipt_id int32) (*models.Receipt, error)
	GetReceiptsByUser(ctx context.Context, user_id int32) ([]*models.Receipt, error)
}

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateReceipt(ctx context.Context, payment_request *models.PaymentRequest) (int32, error) {
	return s.repo.GetExample(id)
}

func (s *Service) GetReceiptByID(ctx context.Context, receipt_id int32) (*models.Receipt, error) {
	return s.repo.GetExample(id)
}

func (s *Service) GetReceiptsByUser(ctx context.Context, user_id int32) ([]*models.Receipt, error) {
	return s.repo.GetExample(id)
}
