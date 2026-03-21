package repository

import models "MrFood/services/review/pkg"

type Repository struct{}

func New() *Repository {
	return &Repository{}
}

// Example method - customize based on your data needs
func (r *Repository) GetExample(id int) (*models.Example, error) {
	// In-memory example - replace with database/Redis
	return &models.Example{ID: id, Name: "Example"}, nil
}
