package middleware

import (
	"context"
	"net/http"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/carissaayo/go-api-gateway/internal/ratelimit"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string, rate float64, burst int) (*ratelimit.Result, error)
}

func RateLimit(limiter RateLimiter, log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			record := GetAPIKey(r.Context())
			if record == nil {
				next.ServeHTTP(w, r)
				return
			}

			rate := float64(record.RateLimit.RequestsPerSecond)
			burst := record.RateLimit.BurstSize
			if rate == 0 {
				rate = 100
			}
			if burst == 0 {
				burst = int(rate) * 2
			}

			result, err := limiter.Allow(r.Context(), record.Key, rate, burst)
			if err != nil {
				log.Error().Err(err).Msg("rate limit check failed")
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))

			if !result.Allowed {
				w.Header().Set("Retry-After", "1")
				writeJSON(w, http.StatusTooManyRequests, `{"error":"rate limit exceeded"}`)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
