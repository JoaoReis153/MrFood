package service

import (
	"MrFood/services/auth/internal/repository"
	models "MrFood/services/auth/pkg"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) StoreUser(user *models.User) (*models.User, error) {
	// TODO: validate mail and uniqueness
	// TODO: hash password
	repo := s.repo
	userId, returnedUsername, err := repo.CreateUser(user.Name, user.Password, user.Email)
	if err != nil {
		return nil, err
	}

	newUser := &models.User{
		ID:   userId,
		Name: returnedUsername,
	}

	return newUser, nil
}
