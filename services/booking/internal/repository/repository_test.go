package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	models "MrFood/services/booking/pkg"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
)

func TestCreateBooking(t *testing.T) {
	ctx := context.Background()

	baseBooking := &models.Booking{
		UserID:       1,
		RestaurantID: 10,
		TimeStart:    time.Now(),
		TimeEnd:      time.Now().Add(time.Hour),
		PeopleCount:  2,
	}

	t.Run("booking already exists", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		repo := New(mock)

		booking := &models.Booking{
			UserID:       1,
			RestaurantID: 1,
			TimeStart:    time.Now(),
			TimeEnd:      time.Now().Add(time.Hour),
			PeopleCount:  2,
		}

		mock.ExpectBegin()

		mock.ExpectQuery(`SELECT 1 FROM booking`).
			WithArgs(booking.RestaurantID, booking.TimeStart, booking.UserID).
			WillReturnRows(pgxmock.NewRows([]string{"1"}).AddRow(int32(1)))

		mock.ExpectRollback()

		_, err = repo.CreateBooking(context.Background(), booking)
		if !errors.Is(err, ErrBookingAlreadyExists) {
			t.Fatalf("expected ErrBookingAlreadyExists, got %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("slots query error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo := New(mock)

		booking := *baseBooking

		mock.ExpectBegin()

		mock.ExpectQuery(`SELECT 1 FROM booking`).
			WithArgs(booking.RestaurantID, booking.TimeStart, booking.UserID).
			WillReturnError(pgx.ErrNoRows)

		mock.ExpectQuery(`SELECT max_slots, current_slots FROM restaurant_slots`).
			WithArgs(booking.RestaurantID, booking.TimeStart).
			WillReturnError(errors.New("db error"))

		mock.ExpectRollback()

		_, err := repo.CreateBooking(ctx, &booking)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("not enough slots", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo := New(mock)

		booking := *baseBooking
		booking.PeopleCount = 20

		mock.ExpectBegin()

		mock.ExpectQuery(`SELECT 1 FROM booking`).
			WithArgs(booking.RestaurantID, booking.TimeStart, booking.UserID).
			WillReturnError(pgx.ErrNoRows)

		mock.ExpectQuery(`SELECT max_slots, current_slots FROM restaurant_slots`).
			WithArgs(booking.RestaurantID, booking.TimeStart).
			WillReturnRows(
				pgxmock.NewRows([]string{"max_slots", "current_slots"}).
					AddRow(15, 10),
			)

		mock.ExpectRollback()

		_, err := repo.CreateBooking(ctx, &booking)
		if !errors.Is(err, ErrInvalidBooking) {
			t.Fatalf("expected ErrInvalidBooking, got %v", err)
		}
	})

	t.Run("no slots row uses default values", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo := New(mock)

		booking := *baseBooking

		mock.ExpectBegin()

		mock.ExpectQuery(`SELECT 1 FROM booking`).
			WithArgs(booking.RestaurantID, booking.TimeStart, booking.UserID).
			WillReturnError(pgx.ErrNoRows)

		mock.ExpectQuery(`SELECT .* FROM restaurant_slots`).
			WithArgs(booking.RestaurantID, booking.TimeStart).
			WillReturnError(pgx.ErrNoRows)

		mock.ExpectQuery(`INSERT INTO booking`).
			WithArgs(booking.UserID, booking.RestaurantID, booking.TimeStart, booking.TimeEnd, booking.PeopleCount).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(42)))

		mock.ExpectCommit()

		id, err := repo.CreateBooking(ctx, &booking)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != 42 {
			t.Fatalf("expected id 42, got %d", id)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("insert error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo := New(mock)

		mock.ExpectBegin()

		mock.ExpectQuery(`SELECT 1 FROM booking`).
			WillReturnError(pgx.ErrNoRows)

		mock.ExpectQuery(`SELECT max_slots, current_slots FROM restaurant_slots`).
			WillReturnError(pgx.ErrNoRows)

		mock.ExpectQuery(`INSERT INTO booking`).
			WillReturnError(errors.New("insert failed"))

		mock.ExpectRollback()

		_, err := repo.CreateBooking(ctx, baseBooking)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo := New(mock)

		mock.ExpectBegin()

		mock.ExpectQuery(`SELECT 1 FROM booking`).
			WillReturnError(pgx.ErrNoRows)

		mock.ExpectQuery(`SELECT max_slots, current_slots FROM restaurant_slots`).
			WillReturnError(pgx.ErrNoRows)

		mock.ExpectQuery(`INSERT INTO booking`).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1))

		mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

		_, err := repo.CreateBooking(ctx, baseBooking)
		if err == nil {
			t.Fatalf("expected commit error")
		}
	})
}

func TestDeleteBooking(t *testing.T) {
	ctx := context.Background()
	baseDeleteBooking := &models.DeleteBooking{
		BookingID: 1,
		UserID:    1,
	}

	t.Run("not found", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo := New(mock)

		mock.ExpectExec(`DELETE FROM booking`).
			WithArgs(baseDeleteBooking.BookingID, baseDeleteBooking.UserID).
			WillReturnResult(pgxmock.NewResult("DELETE", 0))

		err := repo.DeleteBooking(ctx, baseDeleteBooking)
		if !errors.Is(err, ErrBookingNotFound) {
			t.Fatalf("expected ErrBookingNotFound, got %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo := New(mock)

		mock.ExpectExec(`DELETE FROM booking`).
			WillReturnError(errors.New("delete failed"))

		err := repo.DeleteBooking(ctx, baseDeleteBooking)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()

		repo := New(mock)

		mock.ExpectExec(`DELETE FROM booking`).
			WithArgs(baseDeleteBooking.BookingID, baseDeleteBooking.UserID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		err := repo.DeleteBooking(ctx, baseDeleteBooking)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
