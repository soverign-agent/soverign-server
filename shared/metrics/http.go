package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHandler returns an http.Handler that serves the Prometheus exposition
// format for the given registry, with OpenMetrics negotiation enabled.
func NewHandler(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		Registry:          reg,
		EnableOpenMetrics: true,
	})
}
