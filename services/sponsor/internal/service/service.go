package service

import (
	"MrFood/services/sponsor/internal/api/grpc/pb"
	"MrFood/services/sponsor/internal/repository"
	models "MrFood/services/sponsor/pkg"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type Service struct {
	repo             sponsorRepository
	restaurantClient pb.RestaurantToSponsorServiceClient
	paymentClient    pb.PaymentCommandServiceClient
}

func New(repo *repository.Repository, restaurantClient pb.RestaurantToSponsorServiceClient, paymentClient pb.PaymentCommandServiceClient) *Service {
	return &Service{repo: repo, restaurantClient: restaurantClient, paymentClient: paymentClient}
}

type sponsorRepository interface {
	GetRestaurantSponsorship(ctx context.Context, id int64) (*models.SponsorshipResponse, error)
	Sponsor(ctx context.Context, request *models.Sponsorship) (*models.SponsorshipResponse, error)
}

func (s *Service) Sponsor(ctx context.Context, request *models.Sponsorship, owner int64, email string) (*models.SponsorshipResponse, int32, error) {
	restaurant, err := s.getRestaurantDetails(ctx, request.ID)
	if err != nil {
		return nil, 0, err
	}

	slog.Info("RESTAURANT", "data", restaurant)

	if restaurant.OwnerID != owner {
		return nil, 0, errors.New("invalid restaurant owner")
	}

	request.Categories = restaurant.Categories

	res, err := s.repo.Sponsor(ctx, request)
	if err != nil {
		return nil, 0, err
	}

	amount := float32(res.Tier * 20)

	receipt_id, err := s.makePayment(ctx, &models.PaymentRequest{
		UserID:         owner,
		UserEmail:      email,
		IdempotencyKey: GenerateIdempotencyKey(owner, amount, int32(res.ID), "S"),
		Amount:         amount,
		PaymentDescription: fmt.Sprintf("SPONSOR %d FOR RESTAURANT %d WITH TIER %d UNTIL %s",
			res.ID, request.ID, res.Tier, FormatTime(res.Until)),
		PaymentType: "S",
	})
	if err != nil {
		return nil, 0, err
	}

	return res, receipt_id, nil
}

func (s *Service) makePayment(ctx context.Context, req *models.PaymentRequest) (int32, error) {
	res, err := s.paymentClient.MakePayment(ctx, &pb.PaymentRequest{
		UserId:             req.UserID,
		UserEmail:          req.UserEmail,
		Amount:             req.Amount,
		IdempotencyKey:     req.IdempotencyKey,
		PaymentDescription: req.PaymentDescription,
		Type:               req.PaymentType,
	})

	if err != nil {
		slog.Error("failed to get receipt", "error", err)
		return 0, err
	}

	slog.Info("receipt id", "receipt_id", res.ReceiptId)
	return res.ReceiptId, nil
}

func (s *Service) GetRestaurantSponsorship(ctx context.Context, id int64) (*models.SponsorshipResponse, error) {
	return s.repo.GetRestaurantSponsorship(ctx, id)
}

func (s *Service) getRestaurantDetails(ctx context.Context, restaurantID int64) (*models.RestaurantDetails, error) {
	res, err := s.restaurantClient.GetRestaurantSponsorship(ctx, &pb.GetRestaurantSponsorshipRequest{
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

func GenerateIdempotencyKey(userID int64, amount float32, sponsorID int32, service string) string {
	data := fmt.Sprintf("%d:%f:%d:%s", userID, amount, sponsorID, service)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func FormatTime(t time.Time) string {
	return t.UTC().Truncate(30 * time.Minute).Format("2006-01-02T15:04")
}
