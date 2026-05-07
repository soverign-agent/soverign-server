package promql

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL)
	require.NoError(t, err)
	return srv, c
}

func TestNewClient_EmptyBaseURL(t *testing.T) {
	c, err := NewClient("")
	require.Error(t, err)
	assert.Nil(t, c)
}

func TestNewClient_Valid(t *testing.T) {
	c, err := NewClient("http://localhost:9090")
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestQueryInstant_VectorSuccess(t *testing.T) {
	body := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{"metric": {"tenant_id":"t1"}, "value": [1234567890.5, "0.12"]}
			]
		}
	}`
	_, c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/query", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	val, err := c.QueryInstant(context.Background(), `up`, time.Now())
	require.NoError(t, err)
	require.NotNil(t, val)

	vec, ok := val.(model.Vector)
	require.True(t, ok)
	require.Len(t, vec, 1)
	assert.InDelta(t, 0.12, float64(vec[0].Value), 1e-9)
}

func TestQueryRange_MatrixSuccess(t *testing.T) {
	body := `{
		"status": "success",
		"data": {
			"resultType": "matrix",
			"result": [
				{"metric": {"tenant_id":"t1"}, "values": [
					[1234567890.5, "0.12"],
					[1234571490.5, "0.15"]
				]}
			]
		}
	}`
	_, c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/query_range", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	start := time.Unix(1234567890, 0)
	end := time.Unix(1234571490, 0)
	val, err := c.QueryRange(context.Background(), `up`, start, end, time.Hour)
	require.NoError(t, err)

	mat, ok := val.(model.Matrix)
	require.True(t, ok)
	require.Len(t, mat, 1)
	require.Len(t, mat[0].Values, 2)
	assert.InDelta(t, 0.12, float64(mat[0].Values[0].Value), 1e-9)
	assert.InDelta(t, 0.15, float64(mat[0].Values[1].Value), 1e-9)
}

func TestQueryInstant_HTTP500(t *testing.T) {
	_, c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"error","errorType":"internal","error":"boom"}`))
	})

	val, err := c.QueryInstant(context.Background(), `up`, time.Now())
	require.Error(t, err)
	assert.Nil(t, val)
	assert.Contains(t, err.Error(), "instant query")
}

func TestQueryInstant_MalformedJSON(t *testing.T) {
	_, c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not json`))
	})

	val, err := c.QueryInstant(context.Background(), `up`, time.Now())
	require.Error(t, err)
	assert.Nil(t, val)
}

func TestQueryInstant_ContextCancelled(t *testing.T) {
	_, c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	val, err := c.QueryInstant(ctx, `up`, time.Now())
	require.Error(t, err)
	assert.Nil(t, val)
}

func TestQueryRange_NetworkError(t *testing.T) {
	c, err := NewClient("http://127.0.0.1:1") // unreachable
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	val, err := c.QueryRange(ctx, `up`, time.Now().Add(-time.Hour), time.Now(), time.Minute)
	require.Error(t, err)
	assert.Nil(t, val)
}

func TestQueryRange_InvalidStep(t *testing.T) {
	c, err := NewClient("http://localhost:9090")
	require.NoError(t, err)

	val, err := c.QueryRange(context.Background(), `up`, time.Now(), time.Now(), 0)
	require.Error(t, err)
	assert.Nil(t, val)
	assert.Contains(t, err.Error(), "step must be positive")
}
