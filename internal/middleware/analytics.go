package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/carissaayo/go-api-gateway/internal/metrics"
)

type AnalyticsRecorder interface {
	Record(entry AnalyticsEntry)
}

type AnalyticsEntry struct {
	Timestamp  time.Time
	Method     string
	Path       string
	StatusCode int
	Duration   time.Duration
	APIKey     string
	RequestID  string
}

func Analytics(recorder AnalyticsRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &wrappedResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			status := strconv.Itoa(wrapped.statusCode)

			metrics.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
			metrics.RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration.Seconds())

			if wrapped.statusCode == http.StatusTooManyRequests {
				metrics.RateLimitHits.Inc()
			}

			var apiKey string
			if record := GetAPIKey(r.Context()); record != nil {
				apiKey = record.Key
			}

			recorder.Record(AnalyticsEntry{
				Timestamp:  start,
				Method:     r.Method,
				Path:       r.URL.Path,
				StatusCode: wrapped.statusCode,
				Duration:   duration,
				APIKey:     apiKey,
				RequestID:  GetRequestID(r.Context()),
			})
		})
	}
}
