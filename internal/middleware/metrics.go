package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus 指标（复评第三批：/metrics 可观测性）。
// label 用 gin 路由模板（FullPath，如 /api/v1/devices/:id）而非实际路径，避免高基数。
var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dcim_http_requests_total",
			Help: "HTTP 请求总数（method/path 模板/status）",
		},
		[]string{"method", "path", "status"},
	)
	HTTPDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dcim_http_request_duration_seconds",
			Help:    "HTTP 请求时延分布",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)
)

// Metrics 请求指标中间件：计数 + 时延，路径用路由模板防基数爆炸。
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		HTTPDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}
