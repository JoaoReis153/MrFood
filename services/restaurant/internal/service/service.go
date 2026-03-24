package service

import (
	"MrFood/services/restaurant/internal/repository"
	models "MrFood/services/restaurant/pkg"
)

type Service struct {
	repo repository.Repository
}

func New(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service)


