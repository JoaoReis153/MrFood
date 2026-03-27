package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	models "MrFood/services/restaurant/pkg"
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
	if _, err := repo.GetRestaurantStats(ctx, 1); !errors.Is(err, ErrDatabaseNotSet) {
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
