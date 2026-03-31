package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	models "MrFood/services/restaurant/pkg"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

type fakeRows struct {
	values  []time.Time
	nextIdx int
	scanErr error
	iterErr error
	closed  bool
}

func (r *fakeRows) Next() bool {
	return r.nextIdx < len(r.values)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	ptr, ok := dest[0].(*time.Time)
	if !ok {
		return errors.New("invalid destination type")
	}
	*ptr = r.values[r.nextIdx]
	r.nextIdx++
	return nil
}

func (r *fakeRows) Err() error {
	return r.iterErr
}

func (r *fakeRows) Close() {
	r.closed = true
}

func newMockRepo(t *testing.T) (*Repository, pgxmock.PgxPoolIface) {
	t.Helper()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgx mock: %v", err)
	}

	return NewWithDB(mock), mock
}

func expectGetRestaurantByIDQueries(mock pgxmock.PgxPoolIface, id int32, maxSlots int32) {
	mock.ExpectQuery(`(?s)SELECT id, name, latitude, longitude, address, media_url, max_slots, owner_id, owner_name,sponsor_tier\s+FROM restaurants\s+WHERE id = \$1`).
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "latitude", "longitude", "address", "media_url", "max_slots", "owner_id", "owner_name", "sponsor_tier"}).
			AddRow(id, "Nori", 41.0, -8.0, "Somewhere", nil, maxSlots, 5, "owner", 0))

	mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1\s+ORDER BY id`).
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"working_hour"}).
			AddRow(time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC)).
			AddRow(time.Date(2026, 3, 27, 17, 0, 0, 0, time.UTC)))

	mock.ExpectQuery(`(?s)SELECT category\s+FROM restaurant_categories\s+WHERE restaurant_id = \$1\s+ORDER BY id`).
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"category"}).AddRow("sushi").AddRow("japanese"))
}

func TestRepositoryDatabaseNotSet(t *testing.T) {
	repo := &Repository{}
	ctx := context.Background()

	if _, err := repo.GetRestaurantByID(ctx, 1); !errors.Is(err, ErrDatabaseNotSet) {
		t.Fatalf("expected ErrDatabaseNotSet, got %v", err)
	}
	if _, err := repo.GetRestaurantByName(ctx, "a"); !errors.Is(err, ErrDatabaseNotSet) {
		t.Fatalf("expected ErrDatabaseNotSet, got %v", err)
	}
	if _, err := repo.CreateRestaurant(ctx, &models.Restaurant{Name: "a"}); !errors.Is(err, ErrDatabaseNotSet) {
		t.Fatalf("expected ErrDatabaseNotSet, got %v", err)
	}
	if _, err := repo.UpdateRestaurant(ctx, &models.Restaurant{ID: 1}); !errors.Is(err, ErrDatabaseNotSet) {
		t.Fatalf("expected ErrDatabaseNotSet, got %v", err)
	}
	if _, err := repo.GetWorkingHours(ctx, 1, time.Now()); !errors.Is(err, ErrDatabaseNotSet) {
		t.Fatalf("expected ErrDatabaseNotSet, got %v", err)
	}
}

func TestCreateRestaurantRejectsInvalidPayload(t *testing.T) {
	repo := &Repository{}

	if _, err := repo.CreateRestaurant(context.Background(), nil); !errors.Is(err, ErrInvalidRestaurant) {
		t.Fatalf("expected ErrInvalidRestaurant, got %v", err)
	}
	if _, err := repo.CreateRestaurant(context.Background(), &models.Restaurant{Name: " "}); !errors.Is(err, ErrInvalidRestaurant) {
		t.Fatalf("expected ErrInvalidRestaurant, got %v", err)
	}
}

func TestUpdateRestaurantRejectsInvalidPayload(t *testing.T) {
	repo := &Repository{}

	if _, err := repo.UpdateRestaurant(context.Background(), nil); !errors.Is(err, ErrInvalidRestaurant) {
		t.Fatalf("expected ErrInvalidRestaurant, got %v", err)
	}
	if _, err := repo.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 0}); !errors.Is(err, ErrInvalidRestaurant) {
		t.Fatalf("expected ErrInvalidRestaurant, got %v", err)
	}
}

func TestGetRestaurantByIDSuccess(t *testing.T) {
	repo, mock := newMockRepo(t)
	expectGetRestaurantByIDQueries(mock, 1, 50)

	got, err := repo.GetRestaurantByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ID != 1 || got.Name != "Nori" || got.MaxSlots != 50 {
		t.Fatalf("unexpected restaurant: %+v", got)
	}
	if len(got.WorkingHours) != 2 || len(got.Categories) != 2 {
		t.Fatalf("expected working hours and categories to be loaded: %+v", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetRestaurantByIDNotFound(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery(`(?s)SELECT id, name, latitude, longitude, address, media_url, max_slots, owner_id, owner_name,sponsor_tier\s+FROM restaurants\s+WHERE id = \$1`).
		WithArgs(int32(404)).
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.GetRestaurantByID(context.Background(), 404)
	if !errors.Is(err, ErrRestaurantNotFound) {
		t.Fatalf("expected ErrRestaurantNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetRestaurantByIDQueryErrors(t *testing.T) {
	t.Run("working hours query error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`(?s)SELECT id, name, latitude, longitude, address, media_url, max_slots, owner_id, owner_name,sponsor_tier\s+FROM restaurants\s+WHERE id = \$1`).
			WithArgs(int32(1)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "name", "latitude", "longitude", "address", "media_url", "max_slots", "owner_id", "owner_name", "sponsor_tier"}).
				AddRow(int32(1), "Nori", 41.0, -8.0, "Somewhere", nil, int32(10), int32(5), "owner", int32(0)))
		mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1\s+ORDER BY id`).
			WithArgs(int32(1)).
			WillReturnError(errors.New("query failed"))

		_, err := repo.GetRestaurantByID(context.Background(), 1)
		if err == nil || !strings.Contains(err.Error(), "query working hours") {
			t.Fatalf("expected working hours query error, got %v", err)
		}
	})

	t.Run("categories query error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`(?s)SELECT id, name, latitude, longitude, address, media_url, max_slots, owner_id, owner_name,sponsor_tier\s+FROM restaurants\s+WHERE id = \$1`).
			WithArgs(int32(2)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "name", "latitude", "longitude", "address", "media_url", "max_slots", "owner_id", "owner_name", "sponsor_tier"}).
				AddRow(int32(2), "Nori", 41.0, -8.0, "Somewhere", nil, int32(10), int32(5), "owner", int32(0)))
		mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1\s+ORDER BY id`).
			WithArgs(int32(2)).
			WillReturnRows(pgxmock.NewRows([]string{"working_hour"}))
		mock.ExpectQuery(`(?s)SELECT category\s+FROM restaurant_categories\s+WHERE restaurant_id = \$1\s+ORDER BY id`).
			WithArgs(int32(2)).
			WillReturnError(errors.New("query failed"))

		_, err := repo.GetRestaurantByID(context.Background(), 2)
		if err == nil || !strings.Contains(err.Error(), "query categories") {
			t.Fatalf("expected categories query error, got %v", err)
		}
	})
}

func TestGetRestaurantIDPaths(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`(?s)SELECT id\s+FROM restaurants\s+WHERE id = \$1`).WithArgs(int32(11)).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(11)))

		id, err := repo.GetRestaurantID(context.Background(), 11)
		if err != nil || id != 11 {
			t.Fatalf("expected id 11, got id=%d err=%v", id, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`(?s)SELECT id\s+FROM restaurants\s+WHERE id = \$1`).WithArgs(int32(99)).WillReturnError(pgx.ErrNoRows)

		_, err := repo.GetRestaurantID(context.Background(), 99)
		if !errors.Is(err, ErrRestaurantNotFound) {
			t.Fatalf("expected ErrRestaurantNotFound, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`(?s)SELECT id\s+FROM restaurants\s+WHERE id = \$1`).WithArgs(int32(9)).WillReturnError(errors.New("db error"))

		_, err := repo.GetRestaurantID(context.Background(), 9)
		if err == nil || !strings.Contains(err.Error(), "query restaurant ID") {
			t.Fatalf("expected wrapped db error, got %v", err)
		}
	})
}

func TestGetRestaurantByNamePaths(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`(?s)SELECT id\s+FROM restaurants\s+WHERE LOWER\(name\) = LOWER\(\$1\)`).WithArgs("Nori").
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(4)))

		r, err := repo.GetRestaurantByName(context.Background(), "  Nori  ")
		if err != nil || r.ID != 4 {
			t.Fatalf("expected id 4, got %+v err=%v", r, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`(?s)SELECT id\s+FROM restaurants\s+WHERE LOWER\(name\) = LOWER\(\$1\)`).WithArgs("unknown").WillReturnError(pgx.ErrNoRows)

		_, err := repo.GetRestaurantByName(context.Background(), "unknown")
		if !errors.Is(err, ErrRestaurantNotFound) {
			t.Fatalf("expected ErrRestaurantNotFound, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestCreateRestaurantSuccess(t *testing.T) {
	repo, mock := newMockRepo(t)
	model := &models.Restaurant{
		Name:         "Nori",
		Latitude:     41,
		Longitude:    -8,
		Address:      "Somewhere",
		MaxSlots:     30,
		OwnerID:      8,
		OwnerName:    "mario",
		SponsorTier:  1,
		WorkingHours: []string{"2026-03-27T10:00:00Z"},
		Categories:   []string{" sushi ", ""},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO restaurants`).
		WithArgs("Nori", 41.0, -8.0, "Somewhere", pgxmock.AnyArg(), int32(30), int32(8), "mario", int32(1)).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(12)))
	mock.ExpectExec(`(?s)INSERT INTO restaurant_working_hours`).WithArgs(int32(12), time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec(`(?s)INSERT INTO restaurant_categories`).WithArgs(int32(12), "sushi").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	mock.ExpectRollback()

	id, err := repo.CreateRestaurant(context.Background(), model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 12 {
		t.Fatalf("expected id 12, got %d", id)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateRestaurantInvalidTimestampRollsBack(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO restaurants`).
		WithArgs("Nori", 0.0, 0.0, "", pgxmock.AnyArg(), int32(0), int32(1), "mario", int32(0)).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(12)))
	mock.ExpectRollback()

	_, err := repo.CreateRestaurant(context.Background(), &models.Restaurant{
		Name:         "Nori",
		OwnerID:      1,
		OwnerName:    "mario",
		WorkingHours: []string{"bad-ts"},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid timestamp") {
		t.Fatalf("expected invalid timestamp error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateRestaurantNotFound(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int32(7)).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectRollback()

	_, err := repo.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 7, Name: "new"})
	if !errors.Is(err, ErrRestaurantNotFound) {
		t.Fatalf("expected ErrRestaurantNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateRestaurantSuccess(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int32(7)).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec(`(?s)UPDATE restaurants SET`).
		WithArgs("new", "addr", 42.5, -8.6, "https://img", int32(60), int32(7)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec(`(?s)DELETE FROM restaurant_working_hours WHERE restaurant_id = \$1`).WithArgs(int32(7)).
		WillReturnResult(pgxmock.NewResult("DELETE", 2))
	mock.ExpectExec(`(?s)INSERT INTO restaurant_working_hours`).
		WithArgs(int32(7), time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec(`(?s)DELETE FROM restaurant_categories WHERE restaurant_id = \$1`).WithArgs(int32(7)).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectExec(`(?s)INSERT INTO restaurant_categories`).WithArgs(int32(7), "asian").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	expectGetRestaurantByIDQueries(mock, 7, 60)
	mock.ExpectRollback()

	updated, err := repo.UpdateRestaurant(context.Background(), &models.Restaurant{
		ID:           7,
		Name:         "new",
		Address:      "addr",
		Latitude:     42.5,
		Longitude:    -8.6,
		MediaURL:     "https://img",
		MaxSlots:     60,
		WorkingHours: []string{"2026-03-27T09:00:00Z"},
		Categories:   []string{"asian"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.ID != 7 || updated.MaxSlots != 60 {
		t.Fatalf("unexpected update result: %+v", updated)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetWorkingHoursPaths(t *testing.T) {
	t.Run("primary query", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		expectGetRestaurantByIDQueries(mock, 3, 55)
		start := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
		end := start.Add(8 * time.Hour)

		mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1 AND working_hour >= \$2\s+ORDER BY working_hour\s+LIMIT 2`).
			WithArgs(int32(3), start).
			WillReturnRows(pgxmock.NewRows([]string{"working_hour"}).AddRow(start).AddRow(end))

		resp, err := repo.GetWorkingHours(context.Background(), 3, start)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.TimeStart.Equal(start) || !resp.TimeEnd.Equal(end) || resp.MaxSlots != 55 {
			t.Fatalf("unexpected response: %+v", resp)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("fallback query", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		expectGetRestaurantByIDQueries(mock, 4, 10)
		start := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)

		mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1 AND working_hour >= \$2\s+ORDER BY working_hour\s+LIMIT 2`).
			WithArgs(int32(4), start).
			WillReturnRows(pgxmock.NewRows([]string{"working_hour"}))

		fallbackStart := time.Date(2026, 3, 28, 9, 0, 0, 0, time.UTC)
		fallbackEnd := fallbackStart.Add(6 * time.Hour)
		mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1\s+ORDER BY working_hour\s+LIMIT 2`).
			WithArgs(int32(4)).
			WillReturnRows(pgxmock.NewRows([]string{"working_hour"}).AddRow(fallbackStart).AddRow(fallbackEnd))

		resp, err := repo.GetWorkingHours(context.Background(), 4, start)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.TimeStart.Equal(fallbackStart) || !resp.TimeEnd.Equal(fallbackEnd) {
			t.Fatalf("unexpected fallback response: %+v", resp)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestGetWorkingHoursNotFoundWhenNoHours(t *testing.T) {
	repo, mock := newMockRepo(t)
	expectGetRestaurantByIDQueries(mock, 6, 20)
	start := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1 AND working_hour >= \$2\s+ORDER BY working_hour\s+LIMIT 2`).
		WithArgs(int32(6), start).
		WillReturnRows(pgxmock.NewRows([]string{"working_hour"}))
	mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1\s+ORDER BY working_hour\s+LIMIT 2`).
		WithArgs(int32(6)).
		WillReturnRows(pgxmock.NewRows([]string{"working_hour"}))

	_, err := repo.GetWorkingHours(context.Background(), 6, start)
	if !errors.Is(err, ErrRestaurantNotFound) {
		t.Fatalf("expected ErrRestaurantNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRestaurantExists(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int32(3)).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := repo.restaurantExists(context.Background(), 3)
	if err != nil || !exists {
		t.Fatalf("expected exists true, got exists=%v err=%v", exists, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRepositoryNew(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgx mock: %v", err)
	}

	repo := NewWithDB(mock)
	if repo == nil || repo.DB == nil {
		t.Fatal("expected repository DB to be set")
	}
}

func TestRepositoryNewWithNilPool(t *testing.T) {
	repo := New(nil)
	if repo == nil {
		t.Fatal("expected repository instance")
	}
}

func TestCompareRestaurants(t *testing.T) {
	repo, mock := newMockRepo(t)
	expectGetRestaurantByIDQueries(mock, 1, 10)
	expectGetRestaurantByIDQueries(mock, 2, 20)

	r1, r2, err := repo.CompareRestaurants(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r1.ID != 1 || r2.ID != 2 {
		t.Fatalf("unexpected compare result: r1=%+v r2=%+v", r1, r2)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCompareRestaurantsFirstError(t *testing.T) {
	repo := &Repository{}
	_, _, err := repo.CompareRestaurants(context.Background(), 1, 2)
	if !errors.Is(err, ErrDatabaseNotSet) {
		t.Fatalf("expected ErrDatabaseNotSet, got %v", err)
	}
}

func TestCreateRestaurantBeginTxError(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

	_, err := repo.CreateRestaurant(context.Background(), &models.Restaurant{Name: "Nori", OwnerID: 1})
	if err == nil || !strings.Contains(err.Error(), "begin tx") {
		t.Fatalf("expected begin tx error, got %v", err)
	}
}

func TestCreateRestaurantCommitError(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO restaurants`).
		WithArgs("Nori", 0.0, 0.0, "", pgxmock.AnyArg(), int32(0), int32(1), "", int32(0)).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(3)))
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
	mock.ExpectRollback()

	_, err := repo.CreateRestaurant(context.Background(), &models.Restaurant{Name: "Nori", OwnerID: 1})
	if err == nil || !strings.Contains(err.Error(), "commit tx") {
		t.Fatalf("expected commit error, got %v", err)
	}
}

func TestCreateRestaurantExecErrors(t *testing.T) {
	t.Run("working hour insert", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectBegin()
		mock.ExpectQuery(`(?s)INSERT INTO restaurants`).
			WithArgs("Nori", 0.0, 0.0, "", pgxmock.AnyArg(), int32(0), int32(1), "", int32(0)).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(3)))
		mock.ExpectExec(`(?s)INSERT INTO restaurant_working_hours`).WithArgs(int32(3), time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)).
			WillReturnError(errors.New("insert failed"))
		mock.ExpectRollback()

		_, err := repo.CreateRestaurant(context.Background(), &models.Restaurant{Name: "Nori", OwnerID: 1, WorkingHours: []string{"2026-03-27T10:00:00Z"}})
		if err == nil || !strings.Contains(err.Error(), "create working hour") {
			t.Fatalf("expected working hour insert error, got %v", err)
		}
	})

	t.Run("category insert", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectBegin()
		mock.ExpectQuery(`(?s)INSERT INTO restaurants`).
			WithArgs("Nori", 0.0, 0.0, "", pgxmock.AnyArg(), int32(0), int32(1), "", int32(0)).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(3)))
		mock.ExpectExec(`(?s)INSERT INTO restaurant_categories`).WithArgs(int32(3), "sushi").
			WillReturnError(errors.New("insert failed"))
		mock.ExpectRollback()

		_, err := repo.CreateRestaurant(context.Background(), &models.Restaurant{Name: "Nori", OwnerID: 1, Categories: []string{"sushi"}})
		if err == nil || !strings.Contains(err.Error(), "create category") {
			t.Fatalf("expected category insert error, got %v", err)
		}
	})
}

func TestUpdateRestaurantBeginAndExistsErrors(t *testing.T) {
	t.Run("begin tx", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

		_, err := repo.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 1})
		if err == nil || !strings.Contains(err.Error(), "begin tx") {
			t.Fatalf("expected begin tx error, got %v", err)
		}
	})

	t.Run("exists query error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectBegin()
		mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int32(1)).WillReturnError(errors.New("query failed"))
		mock.ExpectRollback()

		_, err := repo.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 1})
		if err == nil || !strings.Contains(err.Error(), "check restaurant") {
			t.Fatalf("expected check restaurant error, got %v", err)
		}
	})
}

func TestUpdateRestaurantMutationErrors(t *testing.T) {
	t.Run("update statement error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectBegin()
		mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int32(1)).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectExec(`(?s)UPDATE restaurants SET`).WillReturnError(errors.New("update failed"))
		mock.ExpectRollback()

		_, err := repo.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 1, Name: "new"})
		if err == nil || !strings.Contains(err.Error(), "update restaurant") {
			t.Fatalf("expected update error, got %v", err)
		}
	})

	t.Run("working hours parse error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectBegin()
		mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int32(1)).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectExec(`(?s)DELETE FROM restaurant_working_hours WHERE restaurant_id = \$1`).WithArgs(int32(1)).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))
		mock.ExpectRollback()

		_, err := repo.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 1, WorkingHours: []string{"bad-ts"}})
		if err == nil || !strings.Contains(err.Error(), "invalid timestamp") {
			t.Fatalf("expected parse timestamp error, got %v", err)
		}
	})
}

func TestGetWorkingHoursFallbackQueryError(t *testing.T) {
	repo, mock := newMockRepo(t)
	expectGetRestaurantByIDQueries(mock, 10, 30)
	start := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1 AND working_hour >= \$2\s+ORDER BY working_hour\s+LIMIT 2`).
		WithArgs(int32(10), start).
		WillReturnRows(pgxmock.NewRows([]string{"working_hour"}))
	mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1\s+ORDER BY working_hour\s+LIMIT 2`).
		WithArgs(int32(10)).
		WillReturnError(errors.New("fallback failed"))

	_, err := repo.GetWorkingHours(context.Background(), 10, start)
	if err == nil || !strings.Contains(err.Error(), "query working hours fallback") {
		t.Fatalf("expected fallback query error, got %v", err)
	}
}

func TestGetWorkingHoursSingleSlot(t *testing.T) {
	repo, mock := newMockRepo(t)
	expectGetRestaurantByIDQueries(mock, 8, 33)
	start := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1 AND working_hour >= \$2\s+ORDER BY working_hour\s+LIMIT 2`).
		WithArgs(int32(8), start).
		WillReturnRows(pgxmock.NewRows([]string{"working_hour"}).AddRow(start))

	resp, err := repo.GetWorkingHours(context.Background(), 8, start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.TimeStart.Equal(resp.TimeEnd) {
		t.Fatalf("expected same start/end for one slot, got %+v", resp)
	}
}

func TestGetWorkingHoursQueryError(t *testing.T) {
	repo, mock := newMockRepo(t)
	expectGetRestaurantByIDQueries(mock, 9, 22)
	start := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)SELECT working_hour\s+FROM restaurant_working_hours\s+WHERE restaurant_id = \$1 AND working_hour >= \$2\s+ORDER BY working_hour\s+LIMIT 2`).
		WithArgs(int32(9), start).
		WillReturnError(errors.New("query failed"))

	_, err := repo.GetWorkingHours(context.Background(), 9, start)
	if err == nil || !strings.Contains(err.Error(), "query working hours from timestamp") {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestRestaurantExistsError(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int32(3)).WillReturnError(errors.New("db down"))

	_, err := repo.restaurantExists(context.Background(), 3)
	if err == nil || !strings.Contains(err.Error(), "check restaurant") {
		t.Fatalf("expected check restaurant error, got %v", err)
	}
}

func TestNullableString(t *testing.T) {
	if got := nullableString(" "); got != nil {
		t.Fatalf("expected nil, got %v", *got)
	}
	value := "abc"
	if got := nullableString(value); got == nil || *got != value {
		t.Fatalf("expected %q pointer, got %v", value, got)
	}
}

func TestParseTimestamp(t *testing.T) {
	rfc := "2026-03-27T10:00:00Z"
	parsed, err := parseTimestamp(rfc)
	if err != nil {
		t.Fatalf("unexpected error parsing RFC3339: %v", err)
	}
	if parsed.UTC().Format(time.RFC3339) != rfc {
		t.Fatalf("expected %s, got %s", rfc, parsed.UTC().Format(time.RFC3339))
	}

	if _, err := parseTimestamp("not-a-timestamp"); err == nil {
		t.Fatal("expected parsing error for invalid timestamp")
	}
}

func TestScanWorkingHoursSuccess(t *testing.T) {
	t1 := time.Date(2026, 3, 27, 8, 0, 0, 0, time.UTC)
	t2 := t1.Add(2 * time.Hour)
	rows := &fakeRows{values: []time.Time{t1, t2}}

	hours, err := scanWorkingHours(rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hours) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(hours))
	}
	if !rows.closed {
		t.Fatal("expected rows.Close to be called")
	}
}

func TestScanWorkingHoursErrors(t *testing.T) {
	scanRows := &fakeRows{values: []time.Time{time.Now()}, scanErr: errors.New("scan failed")}
	if _, err := scanWorkingHours(scanRows); err == nil {
		t.Fatal("expected scan error")
	}

	iterRows := &fakeRows{iterErr: errors.New("iterator failed")}
	if _, err := scanWorkingHours(iterRows); err == nil {
		t.Fatal("expected iterator error")
	}
}
