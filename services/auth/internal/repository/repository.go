package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	DB *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{
		DB: db,
	}
}

func (r *Repository) CreateUser(username, password, email string) (int32, string, error) {
	query := `
		INSERT INTO app_user (username, password, email)
		VALUES ($1, $2, $3)
		RETURNING user_id, username
	`

	var userId int32
	var returnedUsername string

	err := r.DB.QueryRow(context.Background(), query, username, password, email).Scan(&userId, &returnedUsername)

	if err != nil {
		return 0, "", err
	}

	return userId, returnedUsername, nil
}
