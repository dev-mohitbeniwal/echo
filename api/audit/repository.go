// api/audit/repository.go
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type Repository interface {
	LogAccess(ctx context.Context, log AuditLog) error
	QueryLogs(ctx context.Context, from, to time.Time, userID, resourceID string) ([]AuditLog, error)
}

type ElasticsearchRepository struct {
	esClient *elasticsearch.Client
}

// NewElasticsearchRepository creates a new repository with a given Elasticsearch client URL.
func NewElasticsearchRepository(esURL string) (*ElasticsearchRepository, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{esURL},
	}
	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &ElasticsearchRepository{esClient: esClient}, nil
}

// LogAccess logs an audit action to Elasticsearch.
func (r *ElasticsearchRepository) LogAccess(ctx context.Context, log AuditLog) error {
	data, err := json.Marshal(log)
	if err != nil {
		return err
	}

	req := esapi.IndexRequest{
		Index:      "audit-logs",
		DocumentID: fmt.Sprintf("%d-%s", log.Timestamp.Unix(), log.UserID), // Example ID format
		Body:       strings.NewReader(string(data)),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.esClient)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error indexing document: %s", res.String())
	}

	return nil
}

// QueryLogs searches for audit logs in Elasticsearch within a specific time frame and optionally filters by userID and resourceID.
func (r *ElasticsearchRepository) QueryLogs(ctx context.Context, from, to time.Time, userID, resourceID string) ([]AuditLog, error) {
	var buf strings.Builder
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"range": map[string]interface{}{
							"timestamp": map[string]interface{}{
								"gte": from.Format(time.RFC3339),
								"lte": to.Format(time.RFC3339),
							},
						},
					},
				},
			},
		},
	}

	if userID != "" {
		query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"] = append(query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"].([]interface{}), map[string]interface{}{
			"match": map[string]interface{}{
				"user_id": userID,
			},
		})
	}

	if resourceID != "" {
		query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"] = append(query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"].([]interface{}), map[string]interface{}{
			"match": map[string]interface{}{
				"resource_id": resourceID,
			},
		})
	}

	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, err
	}

	res, err := r.esClient.Search(
		r.esClient.Search.WithContext(ctx),
		r.esClient.Search.WithIndex("audit-logs"),
		r.esClient.Search.WithBody(strings.NewReader(buf.String())),
		r.esClient.Search.WithPretty(),
	)

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error searching documents: %s", res.String())
	}

	var rmap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&rmap); err != nil {
		return nil, err
	}

	hits := rmap["hits"].(map[string]interface{})["hits"].([]interface{})
	logs := make([]AuditLog, len(hits))
	for i, hit := range hits {
		source := hit.(map[string]interface{})["_source"]
		data, _ := json.Marshal(source)
		json.Unmarshal(data, &logs[i])
	}

	return logs, nil
}
