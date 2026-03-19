package repository

import models "MrFood/services/gateway/pkg"

type Repository struct{}

func New() *Repository {
	return &Repository{}
}

// Example method - customize based on your data needs
func (r *Repository) GetUserName(id string) (*models.User, error) {
	// In-memory example - replace with database/Redis
	return &models.User{Id: id}, nil
}
