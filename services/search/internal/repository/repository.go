package repository

import (
	"MrFood/services/search/config"
	models "MrFood/services/search/pkg"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strconv"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
)

type Repository struct {
	es    *elasticsearch.Client
	index string
}

func New(ctx context.Context, cfg *config.Config) (*Repository, error) {
	esAddr := fmt.Sprintf("http://%s:%d", cfg.Elastic.Host, cfg.Elastic.Port)

	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{esAddr},
		Username:  cfg.Elastic.Username,
		Password:  cfg.Elastic.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("create elastic client: %w", err)
	}

	infoRes, err := es.Info(es.Info.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("elastic info: %w", err)
	}
	defer func() {
		if err := infoRes.Body.Close(); err != nil {
			slog.Error("error closing response body", "error", err)
		}
	}()

	if infoRes.IsError() {
		body, _ := io.ReadAll(infoRes.Body)
		return nil, fmt.Errorf("elastic info status=%d body=%s", infoRes.StatusCode, string(body))
	}

	if err := ensureSearchIndexExists(ctx, es, cfg.Elastic.Index); err != nil {
		return nil, err
	}

	return &Repository{
		es:    es,
		index: cfg.Elastic.Index,
	}, nil
}

func (r *Repository) Close(_ context.Context) error {
	return nil
}

func (r *Repository) SearchPaginated(ctx context.Context, query models.SearchQuery) (*models.SearchPaginatedResult, error) {
	must := make([]any, 0, 4)
	filter := make([]any, 0, 2)

	if query.Filter.Category != nil {
		must = append(must, map[string]any{
			"term": map[string]any{
				"categories": *query.Filter.Category,
			},
		})
	}

	if query.Filter.FullName != nil {
		must = append(must, map[string]any{
			"match": map[string]any{
				"name": map[string]any{
					"query":    *query.Filter.FullName,
					"operator": "and",
				},
			},
		})
	}

	if query.Filter.NameSuffix != nil {
		must = append(must, map[string]any{
			"wildcard": map[string]any{
				"name.keyword": map[string]any{
					"value":            *query.Filter.NameSuffix + "*",
					"case_insensitive": true,
				},
			},
		})
	}

	if query.Filter.Location != nil {
		filter = append(filter, map[string]any{
			"geo_distance": map[string]any{
				"distance": fmt.Sprintf("%gm", query.Filter.Location.RadiusMeters),
				"location": map[string]any{
					"lat": query.Filter.Location.Latitude,
					"lon": query.Filter.Location.Longitude,
				},
			},
		})
	}

	queryBody := map[string]any{
		"track_total_hits": true,
		"from":             int((query.Page - 1) * query.Limit),
		"size":             int(query.Limit),
		"sort": []map[string]any{
			{"_score": "desc"},
			{"id": "asc"},
		},
	}

	if len(must) == 0 && len(filter) == 0 {
		queryBody["query"] = map[string]any{"match_all": map[string]any{}}
	} else {
		queryBody["query"] = map[string]any{
			"bool": map[string]any{
				"must":   must,
				"filter": filter,
			},
		}
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(queryBody); err != nil {
		return nil, fmt.Errorf("encode elastic query: %w", err)
	}

	searchRes, err := r.es.Search(
		r.es.Search.WithContext(ctx),
		r.es.Search.WithIndex(r.index),
		r.es.Search.WithBody(&buf),
	)
	if err != nil {
		return nil, fmt.Errorf("elastic search: %w", err)
	}
	defer func() {
		if err := searchRes.Body.Close(); err != nil {
			slog.Error("error closing response body", "error", err)
		}
	}()

	if searchRes.IsError() {
		body, _ := io.ReadAll(searchRes.Body)
		return nil, fmt.Errorf("elastic search status=%d body=%s", searchRes.StatusCode, string(body))
	}

	var decoded struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID     string `json:"_id"`
				Source struct {
					ID         int64    `json:"id"`
					Name       string   `json:"name"`
					Latitude   float64  `json:"latitude"`
					Longitude  float64  `json:"longitude"`
					Address    string   `json:"address"`
					Categories []string `json:"categories"`
					MediaURL   *string  `json:"media_url"`
					Location   *struct {
						Lat float64 `json:"lat"`
						Lon float64 `json:"lon"`
					} `json:"location"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(searchRes.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode elastic search response: %w", err)
	}

	data := make([]models.RestaurantSearchResult, 0, len(decoded.Hits.Hits))
	for _, h := range decoded.Hits.Hits {
		id := h.Source.ID
		if id == 0 {
			if parsed, err := strconv.ParseInt(h.ID, 10, 64); err == nil {
				id = parsed
			}
		}

		lat := h.Source.Latitude
		lon := h.Source.Longitude
		if h.Source.Location != nil {
			lat = h.Source.Location.Lat
			lon = h.Source.Location.Lon
		}

		data = append(data, models.RestaurantSearchResult{
			ID:         id,
			Name:       h.Source.Name,
			Latitude:   lat,
			Longitude:  lon,
			Address:    h.Source.Address,
			Categories: h.Source.Categories,
			MediaURL:   h.Source.MediaURL,
		})
	}

	total := int32(decoded.Hits.Total.Value)
	pages := int32(0)
	if query.Limit > 0 {
		pages = int32(math.Ceil(float64(total) / float64(query.Limit)))
	}

	return &models.SearchPaginatedResult{
		Data: data,
		Pagination: models.Pagination{
			Page:  query.Page,
			Limit: query.Limit,
			Total: total,
			Pages: pages,
		},
	}, nil
}

func ensureSearchIndexExists(ctx context.Context, es *elasticsearch.Client, index string) error {
	existsRes, err := es.Indices.Exists([]string{index}, es.Indices.Exists.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("check index exists: %w", err)
	}
	defer func() {
		if err := existsRes.Body.Close(); err != nil {
			slog.Error("error closing response body", "error", err)
		}
	}()

	if existsRes.StatusCode == 200 {
		return nil
	}
	if existsRes.StatusCode == 404 {
		return fmt.Errorf("search index '%s' does not exist (expected to be managed by CDC service)", index)
	}

	if existsRes.StatusCode != 404 {
		body, _ := io.ReadAll(existsRes.Body)
		return fmt.Errorf("check index exists status=%d body=%s", existsRes.StatusCode, string(body))
	}

	return nil
}
