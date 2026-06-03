package service

import (
	"MrFood/services/search/internal/repository"
	models "MrFood/services/search/pkg"
	"context"
	"errors"
	"log/slog"
	"strings"
)

const (
	defaultPage  int32 = 1
	defaultLimit int32 = 20
	maxLimit     int32 = 100
)

var (
	ErrInvalidPagination = errors.New("invalid pagination values")
	ErrInvalidGeoFilter  = errors.New("invalid geospatial filter")
	ErrInvalidTextFilter = errors.New("cannot use name_suffix and full_name together")
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SearchPaginated(ctx context.Context, query models.SearchQuery) (*models.SearchPaginatedResult, error) {
	if query.Page <= 0 {
		query.Page = defaultPage
	}
	if query.Limit <= 0 {
		query.Limit = defaultLimit
	}
	if query.Limit > maxLimit {
		query.Limit = maxLimit
	}

	if query.Page < 1 || query.Limit < 1 {
		slog.WarnContext(ctx, "search rejected: invalid pagination", "page", query.Page, "limit", query.Limit)
		return nil, ErrInvalidPagination
	}

	if query.Filter.NameSuffix != nil && query.Filter.FullName != nil {
		slog.WarnContext(ctx, "search rejected: name_suffix and full_name are mutually exclusive")
		return nil, ErrInvalidTextFilter
	}

	if query.Filter.Location != nil {
		loc := query.Filter.Location
		if loc.RadiusMeters <= 0 || loc.Latitude < -90 || loc.Latitude > 90 || loc.Longitude < -180 || loc.Longitude > 180 {
			slog.WarnContext(ctx, "search rejected: invalid geo filter", "lat", loc.Latitude, "lon", loc.Longitude, "radius_meters", loc.RadiusMeters)
			return nil, ErrInvalidGeoFilter
		}
	}

	trim := func(s *string) *string {
		if s == nil {
			return nil
		}
		v := strings.TrimSpace(*s)
		if v == "" {
			return nil
		}
		return &v
	}

	query.Filter.Category = trim(query.Filter.Category)
	query.Filter.NameSuffix = trim(query.Filter.NameSuffix)
	query.Filter.FullName = trim(query.Filter.FullName)

	return s.repo.SearchPaginated(ctx, query)
}
