package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8"
)

// SearchRepository handles Elasticsearch queries for restaurants
type SearchRepository struct {
	es *elasticsearch.Client
}

// NewSearchRepository creates a new search repository
func NewSearchRepository(es *elasticsearch.Client) *SearchRepository {
	return &SearchRepository{es: es}
}

// SearchQuery represents a search request with filters
type SearchQuery struct {
	FullName     *string
	NameSuffix   *string
	Category     *string
	Latitude     *float64
	Longitude    *float64
	RadiusMeters *float64
	Page         int32
	Limit        int32
}

// SearchResult represents a single search result
type SearchResult struct {
	ID         int32    `json:"id"`
	Name       string   `json:"name"`
	Latitude   float64  `json:"latitude"`
	Longitude  float64  `json:"longitude"`
	Address    string   `json:"address"`
	Categories []string `json:"categories"`
	MediaURL   *string  `json:"media_url,omitempty"`
}

// SearchResponse represents the search response with pagination
type SearchResponse struct {
	Results    []SearchResult `json:"data"`
	Total      int64          `json:"total"`
	Page       int32          `json:"page"`
	Limit      int32          `json:"limit"`
	TotalPages int32          `json:"pages"`
}

// Search performs a paginated search on restaurants
func (sr *SearchRepository) Search(ctx context.Context, query *SearchQuery) (*SearchResponse, error) {
	// Default pagination
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	if query.Page <= 0 {
		query.Page = 1
	}

	// Build Elasticsearch query
	esQuery := sr.buildQuery(query)

	data, err := json.Marshal(esQuery)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	slog.Debug("Elasticsearch query", "query", string(data))

	// Execute search
	res, err := sr.es.Search(
		sr.es.Search.WithContext(ctx),
		sr.es.Search.WithIndex("restaurants"),
		sr.es.Search.WithBody(bytes.NewReader(data)),
		sr.es.Search.WithFrom(int((query.Page-1)*query.Limit)),
		sr.es.Search.WithSize(int(query.Limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.Status())
	}

	// Parse response
	var esResp map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract hits
	hits := esResp["hits"].(map[string]interface{})
	total := int64(hits["total"].(map[string]interface{})["value"].(float64))
	hitsList := hits["hits"].([]interface{})

	results := make([]SearchResult, 0, len(hitsList))
	for _, hit := range hitsList {
		source := hit.(map[string]interface{})["_source"].(map[string]interface{})

		result := SearchResult{
			ID:        int32(source["id"].(float64)),
			Name:      source["name"].(string),
			Latitude:  source["latitude"].(float64),
			Longitude: source["longitude"].(float64),
			Address:   source["address"].(string),
		}

		// Extract optional fields
		if cats, ok := source["categories"].([]interface{}); ok {
			result.Categories = make([]string, len(cats))
			for i, cat := range cats {
				result.Categories[i] = cat.(string)
			}
		}

		if mediaURL, ok := source["media_url"].(string); ok && mediaURL != "" {
			result.MediaURL = &mediaURL
		}

		results = append(results, result)
	}

	totalPages := int32((total + int64(query.Limit) - 1) / int64(query.Limit))

	return &SearchResponse{
		Results:    results,
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
	}, nil
}

// buildQuery constructs the Elasticsearch query
func (sr *SearchRepository) buildQuery(q *SearchQuery) map[string]interface{} {
	must := []map[string]interface{}{}
	filter := []map[string]interface{}{}

	// Text search queries (MUST clauses)
	if q.FullName != nil && *q.FullName != "" {
		must = append(must, map[string]interface{}{
			"match": map[string]interface{}{
				"name": *q.FullName,
			},
		})
	}

	if q.NameSuffix != nil && *q.NameSuffix != "" {
		must = append(must, map[string]interface{}{
			"match": map[string]interface{}{
				"name": map[string]interface{}{
					"query":    *q.NameSuffix,
					"operator": "and",
				},
			},
		})
	}

	// Category filter
	if q.Category != nil && *q.Category != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{
				"categories": *q.Category,
			},
		})
	}

	// Geospatial filter
	if q.Latitude != nil && q.Longitude != nil {
		geoFilter := map[string]interface{}{
			"geo_distance": map[string]interface{}{
				"location": map[string]interface{}{
					"lat": *q.Latitude,
					"lon": *q.Longitude,
				},
			},
		}

		if q.RadiusMeters != nil {
			geoFilter["geo_distance"].(map[string]interface{})["distance"] = fmt.Sprintf("%.0fm", *q.RadiusMeters)
		} else {
			geoFilter["geo_distance"].(map[string]interface{})["distance"] = "5000m" // Default 5km
		}

		filter = append(filter, geoFilter)
	}

	// Build bool query
	boolQuery := map[string]interface{}{}

	if len(must) > 0 {
		if len(must) == 1 {
			boolQuery["must"] = must[0]
		} else {
			boolQuery["must"] = must
		}
	}

	if len(filter) > 0 {
		if len(filter) == 1 {
			boolQuery["filter"] = filter[0]
		} else {
			boolQuery["filter"] = filter
		}
	}

	// If no must or filter, use match_all
	if len(boolQuery) == 0 {
		return map[string]interface{}{
			"query": map[string]interface{}{
				"match_all": map[string]interface{}{},
			},
		}
	}

	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQuery,
		},
	}
}
