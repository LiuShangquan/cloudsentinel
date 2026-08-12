package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type API struct {
	Registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewAPI() *API {
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "cloudsentinel_http_requests_total", Help: "HTTP requests handled by the API."}, []string{"method", "route", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "cloudsentinel_http_request_duration_seconds", Help: "API request duration in seconds.", Buckets: prometheus.DefBuckets}, []string{"method", "route", "status"})
	registry.MustRegister(requests, duration)
	return &API{Registry: registry, requests: requests, duration: duration}
}

func (m *API) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		m.requests.WithLabelValues(c.Request.Method, route, status).Inc()
		m.duration.WithLabelValues(c.Request.Method, route, status).Observe(time.Since(started).Seconds())
	}
}

func (m *API) Handler() http.Handler { return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{}) }
