package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/rs/zerolog"
)

func Recovery(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error().
						Interface("error", err).
						Str("stack", string(debug.Stack())).
						Str("path", r.URL.Path).
						Str("method", r.Method).
						Msg("panic recovered")

					writeJSON(w, http.StatusInternalServerError, `{"error":"internal server error"}`)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
