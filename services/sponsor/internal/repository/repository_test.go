package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	models "MrFood/services/sponsor/pkg"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---- Fakes ----

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (f *fakeRow) Scan(dest ...any) error {
	return f.scanFn(dest...)
}

type fakeTx struct {
	queryRowFn  func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn      func(ctx context.Context, sql string, args ...any) error
	commitErr   error
	rollbackErr error
}

// Begin implements [pgx.Tx].
func (f *fakeTx) Begin(ctx context.Context) (pgx.Tx, error) {
	panic("unimplemented")
}

// Conn implements [pgx.Tx].
func (f *fakeTx) Conn() *pgx.Conn {
	panic("unimplemented")
}

// CopyFrom implements [pgx.Tx].
func (f *fakeTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	panic("unimplemented")
}

// LargeObjects implements [pgx.Tx].
func (f *fakeTx) LargeObjects() pgx.LargeObjects {
	panic("unimplemented")
}

// Prepare implements [pgx.Tx].
func (f *fakeTx) Prepare(ctx context.Context, name string, sql string) (*pgconn.StatementDescription, error) {
	panic("unimplemented")
}

// Query implements [pgx.Tx].
func (f *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	panic("unimplemented")
}

// SendBatch implements [pgx.Tx].
func (f *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	panic("unimplemented")
}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFn != nil {
		return pgconn.CommandTag{}, f.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}
func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return f.queryRowFn(ctx, sql, args...)
}

func (f *fakeDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *fakeTx) Commit(ctx context.Context) error {
	return f.commitErr
}

func (f *fakeTx) Rollback(ctx context.Context) error {
	return f.rollbackErr
}

type fakeDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	beginFn    func(ctx context.Context) (pgx.Tx, error)
}

func (f *fakeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return f.queryRowFn(ctx, sql, args...)
}

func (f *fakeDB) Begin(ctx context.Context) (pgx.Tx, error) {
	return f.beginFn(ctx)
}

// ---- Tests ----

func TestGetRestaurantSponsorship_DatabaseNotSet(t *testing.T) {
	repo := &Repository{}
	_, err := repo.GetRestaurantSponsorship(context.Background(), 1)

	if !errors.Is(err, ErrDatabaseNotSet) {
		t.Fatalf("expected ErrDatabaseNotSet, got %v", err)
	}
}

func TestGetRestaurantSponsorship_NotFound(t *testing.T) {
	repo := New(&fakeDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{
				scanFn: func(dest ...any) error {
					return errors.New("no rows")
				},
			}
		},
	})

	_, err := repo.GetRestaurantSponsorship(context.Background(), 1)

	if !errors.Is(err, ErrSponsorshipNotFound) {
		t.Fatalf("expected ErrSponsorshipNotFound, got %v", err)
	}
}

func TestGetRestaurantSponsorship_Success(t *testing.T) {
	now := time.Now()

	repo := New(&fakeDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*int64)) = 1
					*(dest[1].(*int32)) = 2
					*(dest[2].(*time.Time)) = now
					return nil
				},
			}
		},
	})

	resp, err := repo.GetRestaurantSponsorship(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != 1 || resp.Tier != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestSponsor_InvalidRequest(t *testing.T) {
	repo := New(nil)

	_, err := repo.Sponsor(context.Background(), nil)
	if !errors.Is(err, ErrInvalidSponsorship) {
		t.Fatalf("expected ErrInvalidSponsorship, got %v", err)
	}
}

func TestSponsor_DatabaseNotSet(t *testing.T) {
	repo := &Repository{}

	_, err := repo.Sponsor(context.Background(), &models.Sponsorship{})
	if !errors.Is(err, ErrDatabaseNotSet) {
		t.Fatalf("expected ErrDatabaseNotSet, got %v", err)
	}
}

func TestSponsor_TierNotUpgraded(t *testing.T) {
	repo := New(&fakeDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*int64)) = 1
					*(dest[1].(*int32)) = 3 // existing tier
					*(dest[2].(*time.Time)) = time.Now()
					return nil
				},
			}
		},
	})

	req := &models.Sponsorship{
		ID:   1,
		Tier: 2, // lower than existing
	}

	_, err := repo.Sponsor(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-upgraded tier")
	}
}

func TestSponsor_Success(t *testing.T) {
	now := time.Now()

	tx := &fakeTx{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*int64)) = 1
					*(dest[1].(*int32)) = 5
					*(dest[2].(*time.Time)) = now
					return nil
				},
			}
		},
		execFn: func(ctx context.Context, sql string, args ...any) error {
			return nil
		},
	}

	repo := New(&fakeDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			// simulate no existing sponsorship
			return &fakeRow{
				scanFn: func(dest ...any) error {
					return errors.New("no rows")
				},
			}
		},
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return tx, nil
		},
	})

	req := &models.Sponsorship{
		ID:         1,
		Tier:       5,
		Until:      now,
		Categories: []string{"pizza", "italian"},
	}

	resp, err := repo.Sponsor(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != 1 || resp.Tier != 5 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
