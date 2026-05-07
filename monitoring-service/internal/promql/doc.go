// Package promql wraps the prometheus/client_golang HTTP API in a small
// dependency-injectable surface used by monitoring-service.
//
// The package exposes a Querier interface (instant + range queries) and a
// Client that satisfies it. The interface is deliberately narrow so tests can
// substitute a fake without spinning up an HTTP server.
package promql
