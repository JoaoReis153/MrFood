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

	query_select := `
		SELECT restaurant_id, tier, until
		FROM sponsorship
		WHERE restaurant_id = $1 AND until > NOW();
	`

	sponsorship := &models.SponsorshipResponse{}

	err := r.DB.QueryRow(ctx, query_select, request.ID).Scan(
		&sponsorship.ID,
		&sponsorship.Tier,
		&sponsorship.Until,
	)

	if request.Tier <= sponsorship.Tier {
		return nil, errors.New("Tier can only be upgraded")
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
	//ADD ONLY IF IT DOESNT EXISTS
	query := `
		INSERT INTO sponsorship (restaurant_id, tier, until)
		VALUES ($1, $2, $3)
		ON CONFLICT (restaurant_id)
		DO UPDATE SET
			tier = EXCLUDED.tier,
			until = EXCLUDED.until
		RETURNING restaurant_id, tier, until
	`

	var restaurantId int32
	var tier int32
	var until time.Time

	err = tx.QueryRow(ctx, query, request.ID, request.Tier, request.Until).
		Scan(&restaurantId, &tier, &until)

	if err != nil {
		return nil, fmt.Errorf("create sponsorship: %w", err)
	}

	for _, category := range request.Categories {
		_, err = tx.Exec(ctx, `
			INSERT INTO restaurant_categories (restaurant_id, category)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, request.ID, category)

		if err != nil {
			return nil, fmt.Errorf("insert category: %w", err)
		}
	}

	return &models.SponsorshipResponse{ID: int(restaurantId), Tier: int(tier), Until: until}, nil
}
