package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	models "MrFood/services/review/pkg"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v3"
)

func TestGetReviews(t *testing.T) {
	ctx := context.Background()

	t.Run("no stats row -> empty list", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		restaurantID := int32(10)

		// SELECT review_count ... returns no rows
		mock.ExpectQuery(`SELECT review_count FROM restaurant_stats`).
			WithArgs(restaurantID).
			WillReturnError(pgx.ErrNoRows)

		reviews, total, err := repo.GetReviews(ctx, restaurantID, 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 0 {
			t.Fatalf("expected total 0, got %d", total)
		}
		if len(reviews) != 0 {
			t.Fatalf("expected empty reviews, got %d", len(reviews))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("error on stats query", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		restaurantID := int32(10)

		mock.ExpectQuery(`SELECT review_count FROM restaurant_stats`).
			WithArgs(restaurantID).
			WillReturnError(errors.New("db error"))

		_, _, err = repo.GetReviews(ctx, restaurantID, 1, 10)
		if err == nil || !strings.Contains(err.Error(), "db error") {
			t.Fatalf("expected db error, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("stats exists but zero reviews", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		restaurantID := int32(10)

		rows := pgxmock.NewRows([]string{"review_count"}).AddRow(0)
		mock.ExpectQuery(`SELECT review_count FROM restaurant_stats`).
			WithArgs(restaurantID).
			WillReturnRows(rows)

		reviews, total, err := repo.GetReviews(ctx, restaurantID, 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 0 {
			t.Fatalf("expected total 0, got %d", total)
		}
		if len(reviews) != 0 {
			t.Fatalf("expected empty reviews, got %d", len(reviews))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("error on reviews query", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		restaurantID := int32(10)
		page, limit := 1, 5
		offset := (page - 1) * limit

		rowsCount := pgxmock.NewRows([]string{"review_count"}).AddRow(3)
		mock.ExpectQuery(`SELECT review_count FROM restaurant_stats`).
			WithArgs(restaurantID).
			WillReturnRows(rowsCount)

		mock.ExpectQuery(`SELECT review_id, restaurant_id, user_id, comment, rating, created_at FROM review`).
			WithArgs(restaurantID, limit, offset).
			WillReturnError(errors.New("query failed"))

		_, _, err = repo.GetReviews(ctx, restaurantID, page, limit)
		if err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected query failed error, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("scan error on review rows", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		restaurantID := int32(10)
		page, limit := 1, 5
		offset := (page - 1) * limit

		rowsCount := pgxmock.NewRows([]string{"review_count"}).AddRow(1)
		mock.ExpectQuery(`SELECT review_count FROM restaurant_stats`).
			WithArgs(restaurantID).
			WillReturnRows(rowsCount)

		// Wrong column type or count to trigger scan error
		rows := pgxmock.NewRows([]string{"review_id"}).AddRow(1)
		mock.ExpectQuery(`SELECT review_id, restaurant_id, user_id, comment, rating, created_at FROM review`).
			WithArgs(restaurantID, limit, offset).
			WillReturnRows(rows)

		_, _, err = repo.GetReviews(ctx, restaurantID, page, limit)
		if err == nil {
			t.Fatalf("expected scan error, got nil")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		restaurantID := int32(10)
		page, limit := 1, 2
		offset := (page - 1) * limit

		rowsCount := pgxmock.NewRows([]string{"review_count"}).AddRow(2)
		mock.ExpectQuery(`SELECT review_count FROM restaurant_stats`).
			WithArgs(restaurantID).
			WillReturnRows(rowsCount)

		now := time.Now()
		rows := pgxmock.NewRows(
			[]string{"review_id", "restaurant_id", "user_id", "comment", "rating", "created_at"},
		).AddRow(int32(1), restaurantID, int32(5), "good", int32(4), now).
			AddRow(int32(2), restaurantID, int32(6), "great", int32(5), now.Add(-time.Hour))

		mock.ExpectQuery(`SELECT review_id, restaurant_id, user_id, comment, rating, created_at FROM review`).
			WithArgs(restaurantID, limit, offset).
			WillReturnRows(rows)

		revs, total, err := repo.GetReviews(ctx, restaurantID, page, limit)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 2 {
			t.Fatalf("expected total 2, got %d", total)
		}
		if len(revs) != 2 {
			t.Fatalf("expected 2 reviews, got %d", len(revs))
		}
		if revs[0].ReviewID != 1 || revs[1].ReviewID != 2 {
			t.Fatalf("unexpected reviews: %+v", revs)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestCreateReview(t *testing.T) {
	ctx := context.Background()

	t.Run("unique constraint violation", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		review := models.Review{RestaurantID: 1, UserID: 2, Comment: "x", Rating: 5}

		pgErr := &pgconn.PgError{
			Code:           "23505",
			ConstraintName: "unique_user_restaurant",
		}

		mock.ExpectQuery(`INSERT INTO review`).
			WithArgs(review.RestaurantID, review.UserID, review.Comment, review.Rating).
			WillReturnError(pgErr)

		_, err = repo.CreateReview(ctx, review)
		if !errors.Is(err, models.ErrReviewAlreadyExists) {
			t.Fatalf("expected ErrReviewAlreadyExists, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("other db error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		review := models.Review{RestaurantID: 1, UserID: 2, Comment: "x", Rating: 5}

		mock.ExpectQuery(`INSERT INTO review`).
			WithArgs(review.RestaurantID, review.UserID, review.Comment, review.Rating).
			WillReturnError(errors.New("insert failed"))

		_, err = repo.CreateReview(ctx, review)
		if err == nil || !strings.Contains(err.Error(), "insert failed") {
			t.Fatalf("expected insert failed error, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		review := models.Review{RestaurantID: 1, UserID: 2, Comment: "x", Rating: 5}

		now := time.Now()
		rows := pgxmock.NewRows([]string{"review_id", "created_at"}).
			AddRow(int32(10), now)

		mock.ExpectQuery(`INSERT INTO review`).
			WithArgs(review.RestaurantID, review.UserID, review.Comment, review.Rating).
			WillReturnRows(rows)

		got, err := repo.CreateReview(ctx, review)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ReviewID != 10 {
			t.Fatalf("expected review_id 10, got %d", got.ReviewID)
		}
		if got.CreatedAt.IsZero() {
			t.Fatalf("expected CreatedAt to be set")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestUpdateReview(t *testing.T) {
	ctx := context.Background()

	t.Run("not found", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}

		up := models.UpdateReview{ReviewID: 1, UserID: 7, Comment: nil, Rating: nil}

		mock.ExpectQuery(`UPDATE review`).
			WithArgs(up.Comment, up.Rating, up.ReviewID, up.UserID).
			WillReturnError(pgx.ErrNoRows)

		_, err = repo.UpdateReview(ctx, up)
		if !errors.Is(err, models.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("other db error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}

		up := models.UpdateReview{ReviewID: 1, UserID: 7}
		mock.ExpectQuery(`UPDATE review`).
			WithArgs(up.Comment, up.Rating, up.ReviewID, up.UserID).
			WillReturnError(errors.New("update failed"))

		_, err = repo.UpdateReview(ctx, up)
		if err == nil || !strings.Contains(err.Error(), "update failed") {
			t.Fatalf("expected update failed error, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}

		comment := "new comment"
		rating := int32(4)
		up := models.UpdateReview{ReviewID: 1, UserID: 7, Comment: &comment, Rating: &rating}

		now := time.Now()
		rows := pgxmock.NewRows([]string{"review_id", "restaurant_id", "user_id", "comment", "rating", "created_at"}).
			AddRow(int32(1), int32(2), int32(3), comment, rating, now)

		mock.ExpectQuery(`UPDATE review`).
			WithArgs(up.Comment, up.Rating, up.ReviewID, up.UserID).
			WillReturnRows(rows)

		got, err := repo.UpdateReview(ctx, up)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ReviewID != 1 || got.RestaurantID != 2 || got.UserID != 3 || got.Comment != comment || got.Rating != rating {
			t.Fatalf("unexpected review: %+v", got)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestDeleteReview(t *testing.T) {
	ctx := context.Background()

	t.Run("db error on delete", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}

		mock.ExpectExec(`DELETE FROM review`).
			WithArgs(int32(1), int32(9)).
			WillReturnError(errors.New("delete failed"))

		err = repo.DeleteReview(ctx, 1, 9)
		if err == nil || !strings.Contains(err.Error(), "delete failed") {
			t.Fatalf("expected delete failed error, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("no rows affected -> not found", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}

		mock.ExpectExec(`DELETE FROM review`).
			WithArgs(int32(1), int32(9)).
			WillReturnResult(pgxmock.NewResult("DELETE", 0))

		err = repo.DeleteReview(ctx, 1, 9)
		if !errors.Is(err, models.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}

		mock.ExpectExec(`DELETE FROM review`).
			WithArgs(int32(1), int32(9)).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		err = repo.DeleteReview(ctx, 1, 9)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestGetRestaurantStats(t *testing.T) {
	ctx := context.Background()

	t.Run("no stats row -> zeroed stats", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		restaurantID := int32(5)

		mock.ExpectQuery(`SELECT average_rating, review_count FROM restaurant_stats`).
			WithArgs(restaurantID).
			WillReturnError(pgx.ErrNoRows)

		stats, err := repo.GetRestaurantStats(ctx, restaurantID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.RestaurantID != restaurantID || stats.ReviewCount != 0 || stats.AverageRating != 0 {
			t.Fatalf("unexpected stats: %+v", stats)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error on stats query", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		restaurantID := int32(5)

		mock.ExpectQuery(`SELECT average_rating, review_count FROM restaurant_stats`).
			WithArgs(restaurantID).
			WillReturnError(errors.New("stats failed"))

		_, err = repo.GetRestaurantStats(ctx, restaurantID)
		if err == nil || !strings.Contains(err.Error(), "stats failed") {
			t.Fatalf("expected stats failed error, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := &Repository{db: mock}
		restaurantID := int32(5)

		rows := pgxmock.NewRows([]string{"average_rating", "review_count"}).
			AddRow(4.5, int32(12))

		mock.ExpectQuery(`SELECT average_rating, review_count FROM restaurant_stats`).
			WithArgs(restaurantID).
			WillReturnRows(rows)

		stats, err := repo.GetRestaurantStats(ctx, restaurantID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.RestaurantID != restaurantID || stats.ReviewCount != 12 || stats.AverageRating != 4.5 {
			t.Fatalf("unexpected stats: %+v", stats)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}
