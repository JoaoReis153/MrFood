package repository

import (
	"context"
	"errors"
	"time"

	models "MrFood/services/sponsor/pkg"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSponsorshipNotFound = errors.New("sponsorship not found")
	ErrInvalidSponsorship  = errors.New("invalid sponsorship data")
	ErrDatabaseNotSet      = errors.New("database is not configured")
	ErrDatabaseRollback    = errors.New("database is rollbacked")
)

type Repository struct {
	DB *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) GetRestaurantSponsorship(ctx context.Context, id int32) (*models.SponsorshipResponse, error) {
	return &models.SponsorshipResponse{ID: 1, Tier: 1, Until: time.Now().AddDate(0, 1, 0)}, nil
}

func (r *Repository) Sponsor(ctx context.Context, request *models.SponsorshipRequest) (*models.SponsorshipResponse, error) {
	return &models.SponsorshipResponse{ID: request.ID, Tier: request.Tier, Until: time.Now().AddDate(0, 1, 0)}, nil
}
