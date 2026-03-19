package service

import (
	"MrFood/services/gateway/internal/repository"
	models "MrFood/services/gateway/pkg"
)

type Service struct {
	repo repository.Repository
}

func New(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetUserName(id string) (*models.User, error) {
	return s.repo.GetUserName(id)
}
