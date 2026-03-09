package middleware

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/carissaayo/go-api-gateway/internal/circuitbreaker"
)

type CircuitBreakerManager interface {
	GetCircuitBreaker(backend string) *circuitbreaker.CircuitBreaker
}

func CircuitBreak(manager CircuitBreakerManager, log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			backend := r.Header.Get("X-Gateway-Backend")
			if backend == "" {
				next.ServeHTTP(w, r)
				return
			}

			cb := manager.GetCircuitBreaker(backend)
			if cb == nil {
				next.ServeHTTP(w, r)
				return
			}

			if !cb.Allow() {
				log.Warn().Str("backend", backend).Msg("circuit breaker open")
				writeJSON(w, http.StatusServiceUnavailable, `{"error":"service unavailable"}`)
				return
			}

			wrapped := &wrappedResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(wrapped, r)

			if wrapped.statusCode >= 500 {
				cb.RecordFailure()
			} else {
				cb.RecordSuccess()
			}
		})
	}
}
