package service

import (
	"MrFood/services/sponsor/internal/repository"
	models "MrFood/services/sponsor/pkg"
)

type Service struct {
	repo repository.Repository
}

func New(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetExample(id int) (*models.Example, error) {
	return s.repo.GetExample(id)
}
