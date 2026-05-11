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

func newMockRepo(t *testing.T) (*Repository, pgxmock.PgxPoolIface) {
	t.Helper()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgx mock: %v", err)
	}

	return NewWithDB(mock), mock
}

func expectGetRestaurantByIDQueries(mock pgxmock.PgxPoolIface, id int64, maxSlots int32) {
	mock.ExpectQuery(`(?s)SELECT id, name, latitude, longitude, address,\s+TO_CHAR\(opening_time, 'HH24:MI:SS'\), TO_CHAR\(closing_time, 'HH24:MI:SS'\),\s+media_url, max_slots, owner_id, owner_name, sponsor_tier\s+FROM restaurants\s+WHERE id = \$1`).
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "latitude", "longitude", "address", "opening_time", "closing_time", "media_url", "max_slots", "owner_id", "owner_name", "sponsor_tier"}).
			AddRow(id, "Nori", 41.0, -8.0, "Somewhere", "09:00:00", "17:00:00", nil, maxSlots, int64(5), "owner", int32(0)))

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
	if got.OpeningTime != "09:00:00" || got.ClosingTime != "17:00:00" || len(got.Categories) != 2 {
		t.Fatalf("expected opening/closing time and categories to be loaded: %+v", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetRestaurantByIDNotFound(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery(`(?s)SELECT id, name, latitude, longitude, address,\s+TO_CHAR\(opening_time, 'HH24:MI:SS'\), TO_CHAR\(closing_time, 'HH24:MI:SS'\),\s+media_url, max_slots, owner_id, owner_name, sponsor_tier\s+FROM restaurants\s+WHERE id = \$1`).
		WithArgs(int64(404)).
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
	t.Run("categories query error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`(?s)SELECT id, name, latitude, longitude, address,\s+TO_CHAR\(opening_time, 'HH24:MI:SS'\), TO_CHAR\(closing_time, 'HH24:MI:SS'\),\s+media_url, max_slots, owner_id, owner_name, sponsor_tier\s+FROM restaurants\s+WHERE id = \$1`).
			WithArgs(int64(1)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "name", "latitude", "longitude", "address", "opening_time", "closing_time", "media_url", "max_slots", "owner_id", "owner_name", "sponsor_tier"}).
				AddRow(int64(1), "Nori", 41.0, -8.0, "Somewhere", "09:00:00", "17:00:00", nil, int32(10), int64(5), "owner", int32(0)))
		mock.ExpectQuery(`(?s)SELECT category\s+FROM restaurant_categories\s+WHERE restaurant_id = \$1\s+ORDER BY id`).
			WithArgs(int64(1)).
			WillReturnError(errors.New("query failed"))

		_, err := repo.GetRestaurantByID(context.Background(), 1)
		if err == nil || !strings.Contains(err.Error(), "query categories") {
			t.Fatalf("expected categories query error, got %v", err)
		}
	})
}

func TestGetRestaurantIDPaths(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`(?s)SELECT id\s+FROM restaurants\s+WHERE id = \$1`).WithArgs(int64(11)).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(11)))

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
		mock.ExpectQuery(`(?s)SELECT id\s+FROM restaurants\s+WHERE id = \$1`).WithArgs(int64(99)).WillReturnError(pgx.ErrNoRows)

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
		mock.ExpectQuery(`(?s)SELECT id\s+FROM restaurants\s+WHERE id = \$1`).WithArgs(int64(9)).WillReturnError(errors.New("db error"))

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
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(4)))

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
		Name:        "Nori",
		Latitude:    41,
		Longitude:   -8,
		Address:     "Somewhere",
		OpeningTime: "10:00:00",
		ClosingTime: "20:00:00",
		MaxSlots:    30,
		OwnerID:     8,
		OwnerName:   "mario",
		SponsorTier: 1,
		Categories:  []string{" sushi ", ""},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO restaurants`).
		WithArgs("Nori", 41.0, -8.0, "Somewhere", "10:00:00", "20:00:00", pgxmock.AnyArg(), int32(30), int64(8), "mario", int32(1)).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(12)))
	mock.ExpectExec(`(?s)INSERT INTO restaurant_categories`).WithArgs(int64(12), "sushi").
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

func TestCreateRestaurantInsertRestaurantErrorRollsBack(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO restaurants`).
		WithArgs("Nori", 0.0, 0.0, "", "", "", pgxmock.AnyArg(), int32(0), int64(1), "mario", int32(0)).
		WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()

	_, err := repo.CreateRestaurant(context.Background(), &models.Restaurant{
		Name:      "Nori",
		OwnerID:   1,
		OwnerName: "mario",
	})
	if err == nil || !strings.Contains(err.Error(), "create restaurant") {
		t.Fatalf("expected create restaurant error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateRestaurantNotFound(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int64(7)).
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
	mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int64(7)).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec(`(?s)UPDATE restaurants SET`).
		WithArgs("new", "addr", 42.5, -8.6, "https://img", "09:00:00", "18:00:00", int32(60), int64(7)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec(`(?s)DELETE FROM restaurant_categories WHERE restaurant_id = \$1`).WithArgs(int64(7)).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectExec(`(?s)INSERT INTO restaurant_categories`).WithArgs(int64(7), "asian").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	expectGetRestaurantByIDQueries(mock, 7, 60)
	mock.ExpectRollback()

	updated, err := repo.UpdateRestaurant(context.Background(), &models.Restaurant{
		ID:          7,
		Name:        "new",
		Address:     "addr",
		Latitude:    42.5,
		Longitude:   -8.6,
		MediaURL:    "https://img",
		OpeningTime: "09:00:00",
		ClosingTime: "18:00:00",
		MaxSlots:    60,
		Categories:  []string{"asian"},
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
	t.Run("same day window", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		expectGetRestaurantByIDQueries(mock, 3, 55)
		start := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)

		resp, err := repo.GetWorkingHours(context.Background(), 3, start)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantStart := time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC)
		wantEnd := time.Date(2026, 3, 27, 17, 0, 0, 0, time.UTC)
		if !resp.TimeStart.Equal(wantStart) || !resp.TimeEnd.Equal(wantEnd) || resp.MaxSlots != 55 {
			t.Fatalf("unexpected response: %+v", resp)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("overnight window", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`(?s)SELECT id, name, latitude, longitude, address,\s+TO_CHAR\(opening_time, 'HH24:MI:SS'\), TO_CHAR\(closing_time, 'HH24:MI:SS'\),\s+media_url, max_slots, owner_id, owner_name, sponsor_tier\s+FROM restaurants\s+WHERE id = \$1`).
			WithArgs(int64(4)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "name", "latitude", "longitude", "address", "opening_time", "closing_time", "media_url", "max_slots", "owner_id", "owner_name", "sponsor_tier"}).
				AddRow(int64(4), "Night Bar", 41.0, -8.0, "Somewhere", "22:00:00", "02:00:00", nil, int32(10), int64(5), "owner", int32(0)))
		mock.ExpectQuery(`(?s)SELECT category\s+FROM restaurant_categories\s+WHERE restaurant_id = \$1\s+ORDER BY id`).
			WithArgs(int64(4)).
			WillReturnRows(pgxmock.NewRows([]string{"category"}))
		start := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)

		resp, err := repo.GetWorkingHours(context.Background(), 4, start)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantStart := time.Date(2026, 3, 27, 22, 0, 0, 0, time.UTC)
		wantEnd := time.Date(2026, 3, 28, 2, 0, 0, 0, time.UTC)
		if !resp.TimeStart.Equal(wantStart) || !resp.TimeEnd.Equal(wantEnd) {
			t.Fatalf("unexpected overnight response: %+v", resp)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestGetWorkingHoursNotFoundWhenRestaurantMissing(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery(`(?s)SELECT id, name, latitude, longitude, address,\s+TO_CHAR\(opening_time, 'HH24:MI:SS'\), TO_CHAR\(closing_time, 'HH24:MI:SS'\),\s+media_url, max_slots, owner_id, owner_name, sponsor_tier\s+FROM restaurants\s+WHERE id = \$1`).
		WithArgs(int64(6)).
		WillReturnError(pgx.ErrNoRows)
	start := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)

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
	mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int64(3)).
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
		WithArgs("Nori", 0.0, 0.0, "", "", "", pgxmock.AnyArg(), int32(0), int64(1), "", int32(0)).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(3)))
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
	mock.ExpectRollback()

	_, err := repo.CreateRestaurant(context.Background(), &models.Restaurant{Name: "Nori", OwnerID: 1})
	if err == nil || !strings.Contains(err.Error(), "commit tx") {
		t.Fatalf("expected commit error, got %v", err)
	}
}

func TestCreateRestaurantExecErrors(t *testing.T) {
	t.Run("insert restaurant", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectBegin()
		mock.ExpectQuery(`(?s)INSERT INTO restaurants`).
			WithArgs("Nori", 0.0, 0.0, "", "", "", pgxmock.AnyArg(), int32(0), int64(1), "", int32(0)).
			WillReturnError(errors.New("insert failed"))
		mock.ExpectRollback()

		_, err := repo.CreateRestaurant(context.Background(), &models.Restaurant{Name: "Nori", OwnerID: 1})
		if err == nil || !strings.Contains(err.Error(), "create restaurant") {
			t.Fatalf("expected create restaurant error, got %v", err)
		}
	})

	t.Run("category insert", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectBegin()
		mock.ExpectQuery(`(?s)INSERT INTO restaurants`).
			WithArgs("Nori", 0.0, 0.0, "", "", "", pgxmock.AnyArg(), int32(0), int64(1), "", int32(0)).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(3)))
		mock.ExpectExec(`(?s)INSERT INTO restaurant_categories`).WithArgs(int64(3), "sushi").
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
		mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int64(1)).WillReturnError(errors.New("query failed"))
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
		mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int64(1)).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectExec(`(?s)UPDATE restaurants SET`).WillReturnError(errors.New("update failed"))
		mock.ExpectRollback()

		_, err := repo.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 1, Name: "new"})
		if err == nil || !strings.Contains(err.Error(), "update restaurant") {
			t.Fatalf("expected update error, got %v", err)
		}
	})

	t.Run("update opening time format error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectBegin()
		mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int64(1)).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectExec(`(?s)UPDATE restaurants SET`).WillReturnError(errors.New("update failed"))
		mock.ExpectRollback()

		_, err := repo.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 1, OpeningTime: "bad"})
		if err == nil || !strings.Contains(err.Error(), "update restaurant") {
			t.Fatalf("expected update restaurant error, got %v", err)
		}
	})
}

func TestRestaurantExistsError(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery(`(?s)SELECT EXISTS\(SELECT 1 FROM restaurants WHERE id = \$1\)`).WithArgs(int64(3)).WillReturnError(errors.New("db down"))

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

func TestParseClock(t *testing.T) {
	if _, err := parseClock("09:30:00"); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := parseClock("9:30"); err == nil {
		t.Fatal("expected parse error for invalid clock format")
	}
}
