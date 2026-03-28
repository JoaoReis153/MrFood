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
)

type Repository struct {
	DB *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) GetRestaurantByID(ctx context.Context, id int32) (*models.Restaurant, error) {
	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	query := `
		SELECT id, name, latitude, longitude, address, media_url, max_slots, owner_id, owner_name,sponsor_tier
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

	hoursRows, err := r.DB.Query(ctx, `
		SELECT working_hour
		FROM restaurant_working_hours
		WHERE restaurant_id = $1
		ORDER BY id
	`, id)
	if err != nil {
		return nil, fmt.Errorf("query working hours: %w", err)
	}
	defer hoursRows.Close()

	for hoursRows.Next() {
		var workingHour time.Time
		if err := hoursRows.Scan(&workingHour); err != nil {
			return nil, fmt.Errorf("scan working hour: %w", err)
		}
		restaurant.WorkingHours = append(restaurant.WorkingHours, workingHour.UTC().Format(time.RFC3339))
	}
	if hoursRows.Err() != nil {
		return nil, fmt.Errorf("iterate working hours: %w", hoursRows.Err())
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

func (r *Repository) CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) (int32, error) {
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
	defer func(tx pgx.Tx, ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil {

		}
	}(tx, ctx)

	query := `
		INSERT INTO restaurants (name, latitude, longitude, address, media_url, max_slots, owner_id,owner_name,sponsor_tier)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	var newID int32
	err = tx.QueryRow(ctx, query,
		restaurant.Name,
		restaurant.Latitude,
		restaurant.Longitude,
		restaurant.Address,
		nullableString(restaurant.MediaURL),
		restaurant.MaxSlots,
		restaurant.OwnerID,
		restaurant.OwnerName,
		restaurant.SponsorTier,
	).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("create restaurant: %w", err)
	}

	for _, ts := range restaurant.WorkingHours {
		parsedTS, err := parseTimestamp(ts)
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO restaurant_working_hours (restaurant_id, working_hour) VALUES ($1, $2)`,
			newID,
			parsedTS.UTC(),
		); err != nil {
			return 0, fmt.Errorf("create working hour: %w", err)
		}
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
	defer func(tx pgx.Tx, ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil {

		}
	}(tx, ctx)

	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurants WHERE id = $1)`, restaurant.ID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check restaurant: %w", err)
	}
	if !exists {
		return nil, ErrRestaurantNotFound
	}

	setClauses := make([]string, 0, 8)
	args := make([]any, 0, 9)
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

	if len(restaurant.WorkingHours) > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM restaurant_working_hours WHERE restaurant_id = $1`, restaurant.ID); err != nil {
			return nil, fmt.Errorf("delete working hours: %w", err)
		}
		for _, ts := range restaurant.WorkingHours {
			parsedTS, err := parseTimestamp(ts)
			if err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO restaurant_working_hours (restaurant_id, working_hour) VALUES ($1, $2)`,
				restaurant.ID,
				parsedTS.UTC(),
			); err != nil {
				return nil, fmt.Errorf("insert working hour: %w", err)
			}
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
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return r.GetRestaurantByID(ctx, restaurant.ID)
}

func (r *Repository) CompareRestaurants(ctx context.Context, id1, id2 int32) (*models.Restaurant, *models.Restaurant, error) {
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

func (r *Repository) GetWorkingHours(ctx context.Context, restaurantID int32, timeStart time.Time) (*models.TimeRange, error) {
	if r.DB == nil {
		return nil, ErrDatabaseNotSet
	}

	exists, err := r.restaurantExists(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrRestaurantNotFound
	}

	rows, err := r.DB.Query(ctx, `
		SELECT working_hour
		FROM restaurant_working_hours
		WHERE restaurant_id = $1 AND working_hour >= $2
		ORDER BY working_hour
		LIMIT 2
	`, restaurantID, timeStart.UTC())
	if err != nil {
		return nil, fmt.Errorf("query working hours from timestamp: %w", err)
	}

	hours, err := scanWorkingHours(rows)
	if err != nil {
		return nil, err
	}

	if len(hours) == 0 {
		rows, err = r.DB.Query(ctx, `
			SELECT working_hour
			FROM restaurant_working_hours
			WHERE restaurant_id = $1
			ORDER BY working_hour
			LIMIT 2
		`, restaurantID)
		if err != nil {
			return nil, fmt.Errorf("query working hours fallback: %w", err)
		}

		hours, err = scanWorkingHours(rows)
		if err != nil {
			return nil, err
		}
	}

	if len(hours) == 0 {
		return nil, ErrRestaurantNotFound
	}

	response := &models.TimeRange{TimeStart: hours[0], TimeEnd: hours[0]}
	if len(hours) > 1 {
		response.TimeEnd = hours[1]
	}

	return response, nil
}

func (r *Repository) restaurantExists(ctx context.Context, restaurantID int32) (bool, error) {
	var exists bool
	if err := r.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurants WHERE id = $1)`, restaurantID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check restaurant: %w", err)
	}
	return exists, nil
}

func scanWorkingHours(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}) ([]time.Time, error) {
	defer rows.Close()

	workingHours := make([]time.Time, 0, 2)
	for rows.Next() {
		var value time.Time
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("scan working hour: %w", err)
		}
		workingHours = append(workingHours, value.UTC())
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate working hours: %w", err)
	}

	return workingHours, nil
}

func nullableString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func parseTimestamp(ts string) (time.Time, error) {
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, ts); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q, expected RFC3339", ts)
}
