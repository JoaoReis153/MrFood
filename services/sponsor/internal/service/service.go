package service

import (
	"MrFood/services/sponsor/internal/repository"
	models "MrFood/services/sponsor/pkg"
	"context"
)

type Service struct {
	repo sponsorRepository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

type sponsorRepository interface {
	GetRestaurantSponsorship(ctx context.Context, id int32) (*models.SponsorshipResponse, error)
	Sponsor(ctx context.Context, request *models.Sponsorship) (*models.SponsorshipResponse, error)
}

func (s *Service) Sponsor(ctx context.Context, request *models.Sponsorship) (*models.SponsorshipResponse, error) {
	return s.repo.Sponsor(ctx, request)
}

func (s *Service) GetRestaurantSponsorship(ctx context.Context, id int32) (*models.SponsorshipResponse, error) {
	return s.repo.GetRestaurantSponsorship(ctx, id)
}
