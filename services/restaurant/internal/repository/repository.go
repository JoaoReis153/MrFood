package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	models "MrFood/services/restaurant/pkg"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRestaurantNotFound = errors.New("restaurant not found")
	ErrInvalidRestaurant  = errors.New("invalid restaurant data")
	ErrDatabaseNotSet     = errors.New("database is not configured")
	ErrDatabaseRollback   = errors.New("database is rollbacked")
)

type Repository struct {
	DB dbExecutor
}

type dbExecutor interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{DB: db}
}

func NewWithDB(db dbExecutor) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) GetRestaurantByID(ctx context.Context, id int64) (*models.Restaurant, error) {
	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	query := `
		SELECT id, name, latitude, longitude, address,
		       TO_CHAR(opening_time, 'HH24:MI:SS'), TO_CHAR(closing_time, 'HH24:MI:SS'),
		       media_url, max_slots, owner_id, owner_name, sponsor_tier
		FROM restaurants
		WHERE id = $1
	`

	restaurant := &models.Restaurant{}
	var mediaURL *string

	err := r.DB.QueryRow(ctx, query, id).Scan(
		&restaurant.ID,
		&restaurant.Name,
		&restaurant.Latitude,
		&restaurant.Longitude,
		&restaurant.Address,
		&restaurant.OpeningTime,
		&restaurant.ClosingTime,
		&mediaURL,
		&restaurant.MaxSlots,
		&restaurant.OwnerID,
		&restaurant.OwnerName,
		&restaurant.SponsorTier,
	)
	if err != nil {
		return nil, ErrRestaurantNotFound
	}

	if mediaURL != nil {
		restaurant.MediaURL = *mediaURL
	}

	categoriesRows, err := r.DB.Query(ctx, `
		SELECT category
		FROM restaurant_categories
		WHERE restaurant_id = $1
		ORDER BY id
	`, id)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer categoriesRows.Close()

	for categoriesRows.Next() {
		var category string
		if err := categoriesRows.Scan(&category); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		restaurant.Categories = append(restaurant.Categories, category)
	}
	if categoriesRows.Err() != nil {
		return nil, fmt.Errorf("iterate categories: %w", categoriesRows.Err())
	}

	return restaurant, nil
}

func (r *Repository) GetRestaurantID(ctx context.Context, id int64) (int64, error) {
	if r.DB == nil {
		return 0, ErrDatabaseNotSet
	}
	query := `
		SELECT id
		FROM restaurants
		WHERE id = $1
	`
	var restaurantID int64
	err := r.DB.QueryRow(ctx, query, id).Scan(&restaurantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrRestaurantNotFound
		}
		return 0, fmt.Errorf("query restaurant ID: %w", err)
	}
	return restaurantID, nil
}

func (r *Repository) GetRestaurantByName(ctx context.Context, name string) (*models.Restaurant, error) {
	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	query := `
		SELECT id
		FROM restaurants
		WHERE LOWER(name) = LOWER($1)
	`

	restaurant := &models.Restaurant{}
	if err := r.DB.QueryRow(ctx, query, strings.TrimSpace(name)).Scan(&restaurant.ID); err != nil {
		return nil, ErrRestaurantNotFound
	}

	return restaurant, nil
}

func (r *Repository) CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) (int64, error) {
	if restaurant == nil || strings.TrimSpace(restaurant.Name) == "" {
		return 0, ErrInvalidRestaurant
	}
	if r.DB == nil {
		return 0, ErrDatabaseNotSet
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	query := `
		INSERT INTO restaurants (id, name, latitude, longitude, address, opening_time, closing_time, media_url, max_slots, owner_id, owner_name, sponsor_tier)
		VALUES (
			COALESCE(
				(SELECT MAX(id) + 1 FROM restaurants),
				1
			),
			$1, $2, $3, $4, $5::time, $6::time, $7, $8, $9, $10, $11
		)
		RETURNING id
	`

	var newID int64
	err = tx.QueryRow(ctx, query,
		restaurant.Name,
		restaurant.Latitude,
		restaurant.Longitude,
		restaurant.Address,
		restaurant.OpeningTime,
		restaurant.ClosingTime,
		nullableString(restaurant.MediaURL),
		restaurant.MaxSlots,
		restaurant.OwnerID,
		restaurant.OwnerName,
		restaurant.SponsorTier,
	).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("create restaurant: %w", err)
	}

	for _, category := range restaurant.Categories {
		trimmed := strings.TrimSpace(category)
		if trimmed == "" {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO restaurant_categories (restaurant_id, category) VALUES ($1, $2)`,
			newID,
			trimmed,
		); err != nil {
			return 0, fmt.Errorf("create category: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		_ = tx.Rollback(ctx)
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return newID, nil
}

func (r *Repository) UpdateRestaurant(ctx context.Context, restaurant *models.Restaurant) (*models.Restaurant, error) {
	if restaurant == nil || restaurant.ID == 0 {
		return nil, ErrInvalidRestaurant
	}
	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurants WHERE id = $1)`, restaurant.ID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check restaurant: %w", err)
	}
	if !exists {
		return nil, ErrRestaurantNotFound
	}

	setClauses := make([]string, 0, 10)
	args := make([]any, 0, 11)
	argPos := 1

	if restaurant.Name != "" {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argPos))
		args = append(args, restaurant.Name)
		argPos++
	}
	if restaurant.Address != "" {
		setClauses = append(setClauses, fmt.Sprintf("address = $%d", argPos))
		args = append(args, restaurant.Address)
		argPos++
	}
	if restaurant.Latitude != 0 {
		setClauses = append(setClauses, fmt.Sprintf("latitude = $%d", argPos))
		args = append(args, restaurant.Latitude)
		argPos++
	}
	if restaurant.Longitude != 0 {
		setClauses = append(setClauses, fmt.Sprintf("longitude = $%d", argPos))
		args = append(args, restaurant.Longitude)
		argPos++
	}
	if restaurant.MediaURL != "" {
		setClauses = append(setClauses, fmt.Sprintf("media_url = $%d", argPos))
		args = append(args, restaurant.MediaURL)
		argPos++
	}
	if restaurant.OpeningTime != "" {
		setClauses = append(setClauses, fmt.Sprintf("opening_time = $%d::time", argPos))
		args = append(args, restaurant.OpeningTime)
		argPos++
	}
	if restaurant.ClosingTime != "" {
		setClauses = append(setClauses, fmt.Sprintf("closing_time = $%d::time", argPos))
		args = append(args, restaurant.ClosingTime)
		argPos++
	}
	if restaurant.MaxSlots > 0 {
		setClauses = append(setClauses, fmt.Sprintf("max_slots = $%d", argPos))
		args = append(args, restaurant.MaxSlots)
		argPos++
	}

	if len(setClauses) > 0 {
		args = append(args, restaurant.ID)
		updateQuery := fmt.Sprintf("UPDATE restaurants SET %s WHERE id = $%d", strings.Join(setClauses, ", "), argPos)
		if _, err := tx.Exec(ctx, updateQuery, args...); err != nil {
			return nil, fmt.Errorf("update restaurant: %w", err)
		}
	}

	if len(restaurant.Categories) > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM restaurant_categories WHERE restaurant_id = $1`, restaurant.ID); err != nil {
			return nil, fmt.Errorf("delete categories: %w", err)
		}
		for _, category := range restaurant.Categories {
			trimmed := strings.TrimSpace(category)
			if trimmed == "" {
				continue
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO restaurant_categories (restaurant_id, category) VALUES ($1, $2)`,
				restaurant.ID,
				trimmed,
			); err != nil {
				return nil, fmt.Errorf("insert category: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return r.GetRestaurantByID(ctx, restaurant.ID)
}

func (r *Repository) CompareRestaurants(ctx context.Context, id1, id2 int64) (*models.Restaurant, *models.Restaurant, error) {
	restaurant1, err := r.GetRestaurantByID(ctx, id1)
	if err != nil {
		return nil, nil, err
	}

	restaurant2, err := r.GetRestaurantByID(ctx, id2)
	if err != nil {
		return nil, nil, err
	}

	return restaurant1, restaurant2, nil
}

func (r *Repository) GetWorkingHours(ctx context.Context, restaurantID int64, timeStart time.Time) (*models.WorkingHoursResponse, error) {
	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	restaurant, err := r.GetRestaurantByID(ctx, restaurantID)
	if err != nil {
		return nil, err
	}

	openingTime, err := parseClock(restaurant.OpeningTime)
	if err != nil {
		return nil, fmt.Errorf("parse opening time: %w", err)
	}
	closingTime, err := parseClock(restaurant.ClosingTime)
	if err != nil {
		return nil, fmt.Errorf("parse closing time: %w", err)
	}

	base := timeStart.UTC()
	if base.IsZero() {
		base = time.Now().UTC()
	}

	start := time.Date(base.Year(), base.Month(), base.Day(), openingTime.Hour(), openingTime.Minute(), openingTime.Second(), 0, time.UTC)
	end := time.Date(base.Year(), base.Month(), base.Day(), closingTime.Hour(), closingTime.Minute(), closingTime.Second(), 0, time.UTC)
	if !end.After(start) {
		end = end.Add(24 * time.Hour)
	}

	return &models.WorkingHoursResponse{TimeStart: start, TimeEnd: end, MaxSlots: restaurant.MaxSlots}, nil
}

func (r *Repository) restaurantExists(ctx context.Context, restaurantID int64) (bool, error) {
	var exists bool
	if err := r.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurants WHERE id = $1)`, restaurantID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check restaurant: %w", err)
	}
	return exists, nil
}

func nullableString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func parseClock(value string) (time.Time, error) {
	parsed, err := time.Parse(time.TimeOnly, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid clock value %q", value)
	}
	return parsed, nil
}
