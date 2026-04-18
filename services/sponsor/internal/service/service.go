package service

import (
	"MrFood/services/sponsor/internal/api/grpc/pb"
	"MrFood/services/sponsor/internal/repository"
	models "MrFood/services/sponsor/pkg"
	"context"
	"errors"
	"log/slog"
)

type Service struct {
	repo   sponsorRepository
	client pb.RestaurantToSponsorServiceClient
}

func New(repo *repository.Repository, client pb.RestaurantToSponsorServiceClient) *Service {
	return &Service{repo: repo, client: client}
}

type sponsorRepository interface {
	GetRestaurantSponsorship(ctx context.Context, id int64) (*models.SponsorshipResponse, error)
	Sponsor(ctx context.Context, request *models.Sponsorship) (*models.SponsorshipResponse, error)
}

func (s *Service) Sponsor(ctx context.Context, request *models.Sponsorship, owner int64) (*models.SponsorshipResponse, error) {
	restaurant, err := s.getRestaurantDetails(ctx, request.ID)
	if err != nil {
		return nil, err
	}

	slog.Info("RESTAURANT", "data", restaurant)

	if restaurant.OwnerID != owner {
		return nil, errors.New("invalid restaurant owner")
	}

	request.Categories = restaurant.Categories

	return s.repo.Sponsor(ctx, request)
}

func (s *Service) GetRestaurantSponsorship(ctx context.Context, id int64) (*models.SponsorshipResponse, error) {
	return s.repo.GetRestaurantSponsorship(ctx, id)
}

func (s *Service) getRestaurantDetails(ctx context.Context, restaurantID int64) (*models.RestaurantDetails, error) {
	res, err := s.client.GetRestaurantSponsorship(ctx, &pb.GetRestaurantSponsorshipRequest{
		Id: restaurantID,
	})

	if err != nil {
		return nil, err
	}

	return &models.RestaurantDetails{
		ID:         res.Id,
		Categories: res.Categories,
		OwnerID:    res.OwnerId,
	}, nil
}
