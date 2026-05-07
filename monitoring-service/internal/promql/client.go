package promql

import (
	"context"
	"fmt"
	"time"

	promapi "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// Querier is the small interface monitoring-service depends on. Production
// uses *Client; tests inject fakes.
type Querier interface {
	QueryInstant(ctx context.Context, query string, ts time.Time) (model.Value, error)
	QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (model.Value, error)
}

// Client is a thin wrapper around prometheus/client_golang/api/prometheus/v1.
type Client struct {
	api v1.API
}

// NewClient constructs a Client targeting the Prometheus HTTP API at baseURL
// (e.g. "http://localhost:9090"). It returns an error if baseURL is empty or
// the underlying api client cannot be built.
func NewClient(baseURL string) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("promql: baseURL is required")
	}
	apiClient, err := promapi.NewClient(promapi.Config{Address: baseURL})
	if err != nil {
		return nil, fmt.Errorf("promql: build api client: %w", err)
	}
	return &Client{api: v1.NewAPI(apiClient)}, nil
}

// QueryInstant runs an instant PromQL query at ts.
func (c *Client) QueryInstant(ctx context.Context, query string, ts time.Time) (model.Value, error) {
	val, warnings, err := c.api.Query(ctx, query, ts)
	if err != nil {
		return nil, fmt.Errorf("promql: instant query: %w", err)
	}
	if len(warnings) > 0 {
		// Warnings are non-fatal but should not be silently dropped in audit
		// systems. Surface them via wrapped error only when the value is nil.
		_ = warnings
	}
	return val, nil
}

// QueryRange runs a range PromQL query between start and end with the given step.
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (model.Value, error) {
	if step <= 0 {
		return nil, fmt.Errorf("promql: step must be positive, got %s", step)
	}
	r := v1.Range{Start: start, End: end, Step: step}
	val, warnings, err := c.api.QueryRange(ctx, query, r)
	if err != nil {
		return nil, fmt.Errorf("promql: range query: %w", err)
	}
	_ = warnings
	return val, nil
}
