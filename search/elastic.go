package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"go-cqrs/models"

	"github.com/elastic/go-elasticsearch/v7"
)

type ElasticSearchRepository struct {
	client *elasticsearch.Client
}

func NewElasticSearchRepository(url string) (*ElasticSearchRepository, error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{url}})
	if err != nil {
		return nil, err
	}

	return &ElasticSearchRepository{client: client}, nil
}

func (r *ElasticSearchRepository) Close() {
	// no close function for elasticsearch
	return
}

func (r *ElasticSearchRepository) IndexFeed(ctx context.Context, feed models.Feed) error {
	body, _ := json.Marshal(feed)
	_, err := r.client.Index("feed", bytes.NewBuffer(body),
		r.client.Index.WithDocumentID(feed.ID),
		r.client.Index.WithContext(ctx),
		r.client.Index.WithRefresh("wait_for"),
	)
	return err
}

func (r *ElasticSearchRepository) SearchFeed(ctx context.Context, query string) ([]models.Feed, error) {
	var buf bytes.Buffer

	searchQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":            query,
				"fields":           []string{"title", "description"},
				"fuzziness":        3,
				"cutoff_frequency": 0.0001,
			},
		},
	}

	if err := json.NewEncoder(&buf).Encode(searchQuery); err != nil {
		return nil, err
	}

	res, err := r.client.Search(
		r.client.Search.WithContext(ctx),
		r.client.Search.WithIndex("feeds"),
		r.client.Search.WithBody(&buf),
		r.client.Search.WithTrackTotalHits(true))
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			// TODO: add log here
		}
	}()

	if res.IsError() {
		return nil, errors.New(res.String())
	}

	var eRes map[string]interface{}
	if err = json.NewDecoder(res.Body).Decode(&eRes); err != nil {
		return nil, err
	}

	var feeds []models.Feed
	for _, hit := range eRes["hits"].(map[string]interface{})["hits"].([]interface{}) {
		var feed models.Feed
		source := hit.(map[string]interface{})["_source"]
		jsonResult, err := json.Marshal(source)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(jsonResult, &feed); err != nil {
			return nil, err
		}

		feeds = append(feeds, feed)
	}

	return feeds, nil
}
