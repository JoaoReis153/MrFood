package service

import (
	"MrFood/services/review/internal/repository"
	models "MrFood/services/review/pkg"
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
