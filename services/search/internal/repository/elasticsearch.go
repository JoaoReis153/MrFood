package repository

import (
	"MrFood/services/search/config"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8"
)

const indexName = "restaurants"

// ElasticsearchClient wraps Elasticsearch connection
type ElasticsearchClient struct {
	client *elasticsearch.Client
}

// RestaurantDocument represents a restaurant in Elasticsearch
type RestaurantDocument struct {
	ID          int32    `json:"id"`
	Name        string   `json:"name"`
	Latitude    float64  `json:"latitude"`
	Longitude   float64  `json:"longitude"`
	Location    GeoPoint `json:"location"`
	Address     string   `json:"address"`
	Categories  []string `json:"categories"`
	MediaURL    *string  `json:"media_url,omitempty"`
	MaxSlots    int32    `json:"max_slots"`
	OwnerID     int32    `json:"owner_id"`
	OwnerName   string   `json:"owner_name"`
	SponsorTier int32    `json:"sponsor_tier"`
}

// GeoPoint for geographical coordinates
type GeoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// NewElasticsearchClient creates a new Elasticsearch client
func NewElasticsearchClient(cfg *config.Config) (*ElasticsearchClient, error) {
	addr := fmt.Sprintf("http://%s:%d", cfg.Elasticsearch.Host, cfg.Elasticsearch.Port)

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{addr},
	})
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("verify connection: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch error: %s", res.Status())
	}

	slog.Info("Connected to Elasticsearch")
	return &ElasticsearchClient{client: client}, nil
}

// InitializeIndex creates the index with proper mappings
func (ec *ElasticsearchClient) InitializeIndex(ctx context.Context) error {
	res, err := ec.client.Indices.Exists([]string{indexName})
	if err != nil {
		return fmt.Errorf("check index: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		slog.Info("Index already exists", "index", indexName)
		return nil
	}

	mapping := map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id": map[string]interface{}{"type": "keyword"},
				"name": map[string]interface{}{
					"type": "text",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{"type": "keyword"},
					},
				},
				"latitude":     map[string]interface{}{"type": "double"},
				"longitude":    map[string]interface{}{"type": "double"},
				"location":     map[string]interface{}{"type": "geo_point"},
				"address":      map[string]interface{}{"type": "text"},
				"categories":   map[string]interface{}{"type": "keyword"},
				"media_url":    map[string]interface{}{"type": "keyword"},
				"max_slots":    map[string]interface{}{"type": "integer"},
				"owner_id":     map[string]interface{}{"type": "keyword"},
				"owner_name":   map[string]interface{}{"type": "text"},
				"sponsor_tier": map[string]interface{}{"type": "integer"},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(mapping); err != nil {
		return fmt.Errorf("encode mapping: %w", err)
	}

	res, err = ec.client.Indices.Create(indexName, ec.client.Indices.Create.WithBody(&buf))
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("index error: %s", res.Status())
	}

	slog.Info("Index created successfully", "index", indexName)
	return nil
}

// IndexRestaurant indexes a single restaurant document
func (ec *ElasticsearchClient) IndexRestaurant(ctx context.Context, doc *RestaurantDocument) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}

	res, err := ec.client.Index(
		indexName,
		ec.client.Index.WithDocumentID(fmt.Sprintf("%d", doc.ID)),
		ec.client.Index.WithContext(ctx),
		ec.client.Index.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return fmt.Errorf("index document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("index error: %s", res.Status())
	}

	return nil
}

// DeleteRestaurant deletes a restaurant document
func (ec *ElasticsearchClient) DeleteRestaurant(ctx context.Context, restaurantID int32) error {
	res, err := ec.client.Delete(indexName, fmt.Sprintf("%d", restaurantID))
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("delete error: %s", res.Status())
	}

	return nil
}
