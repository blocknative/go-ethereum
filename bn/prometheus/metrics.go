package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metrics struct {
	registry  *prometheus.Registry
	gatherers prometheus.Gatherers
}

var Metrics metrics

func init() {
	reg := prometheus.NewRegistry()
	Metrics = metrics{registry: reg, gatherers: prometheus.Gatherers{reg}}
}

func (m *metrics) Register(collector prometheus.Collector) error {
	return m.registry.Register(collector)
}

func (m *metrics) Handler() http.Handler {
	return promhttp.HandlerFor(
		m.gatherers[0],
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	)
}
