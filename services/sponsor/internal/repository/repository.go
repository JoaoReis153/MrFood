package repository

import (
	"context"
	"errors"
	"fmt"
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

	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	query := `
		SELECT restaurant_id, tier, until
		FROM sponsorship
		WHERE restaurant_id = $1 AND until > NOW();
	`

	sponsorship := &models.SponsorshipResponse{}

	err := r.DB.QueryRow(ctx, query, id).Scan(
		&sponsorship.ID,
		&sponsorship.Tier,
		&sponsorship.Until,
	)

	if err != nil {
		return nil, ErrSponsorshipNotFound
	}

	return sponsorship, nil
}

func (r *Repository) Sponsor(ctx context.Context, request *models.Sponsorship) (*models.SponsorshipResponse, error) {
	if request == nil {
		return nil, ErrInvalidSponsorship
	}
	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		} else {
			err = tx.Commit(ctx)
		}
	}()

	query := `
		INSERT INTO sponsorship (restaurant_id, tier, until)
		VALUES ($1, $2, $3)
		RETURNING restaurant_id, tier, until
	`

	var restaurantId int32
	var tier int32
	var until time.Time

	err = tx.QueryRow(ctx, query, request.ID, request.Tier, time.Now().AddDate(0, 1, 0)).
		Scan(&restaurantId, &tier, &until)

	if err != nil {
		return nil, fmt.Errorf("create restaurant: %w", err)
	}
	return &models.SponsorshipResponse{ID: int(restaurantId), Tier: int(tier), Until: until}, nil
}
