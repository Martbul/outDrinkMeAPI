package middleware

import (
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method"},
	)
	authRejections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_rejections_total",
			Help: "Total number of unauthorized requests",
		},
		[]string{"reason"},
	)
)

// InitPrometheus registers the metrics. Call this from main.go
func InitPrometheus() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(authRejections)
}

// MonitorMiddleware wraps the router to track all request stats
func MonitorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Initialize with 200 OK in case WriteHeader isn't called explicitly
		ww := &responseWriter{w, http.StatusOK}

		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()

		// Log data to Prometheus
		// We use r.URL.Path. Be careful: if you have IDs in paths (e.g. /user/123),
		// this creates too many metrics. For Gorilla Mux, it is often better 
		// to use `mux.CurrentRoute(r).GetPathTemplate()` in production, 
		// but r.URL.Path is fine for now.
		httpRequestsTotal.WithLabelValues(r.URL.Path, r.Method, http.StatusText(ww.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.URL.Path, r.Method).Observe(duration)

		// Track Auth failures specifically
		if ww.statusCode == http.StatusUnauthorized {
			authRejections.WithLabelValues("401_unauthorized").Inc()
		} else if ww.statusCode == http.StatusForbidden {
			authRejections.WithLabelValues("403_forbidden").Inc()
		}
	})
}

// BasicAuthMiddleware protects /metrics
func BasicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		
		metricsUser := os.Getenv("METRICS_USER")
		metricsPass := os.Getenv("METRICS_PASS")

		if !ok || user != metricsUser || pass != metricsPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Metrics"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// PprofSecurityMiddleware protects /debug/pprof
func PprofSecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check against Render Env Var
		if r.Header.Get("X-Pprof-Secret") != os.Getenv("PPROF_SECRET") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}