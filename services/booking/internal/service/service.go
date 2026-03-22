package service

import (
	"MrFood/services/booking/internal/repository"
	models "MrFood/services/booking/pkg"
	"context"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateBooking(ctx context.Context, booking *models.Booking) (*models.Booking, error) {
	return nil, nil
}
