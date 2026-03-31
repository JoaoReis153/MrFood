package repository

import (
	"MrFood/services/payment/config"
	models "MrFood/services/payment/pkg"
	"context"
)

type Repository struct{}

func New(_ context.Context, _ *config.Config) (*Repository, error) {
	return &Repository{}, nil
}

func (r *Repository) Close(_ context.Context) error {
	return nil
}

func (r *Repository) CreateReceipt(payment_request *models.PaymentRequest) (int32, error) {
	return &models.Example{ID: id, Name: "Example"}, nil
}
