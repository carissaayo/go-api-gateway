package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type APIKeyRateLimit struct {
	RequestsPerSecond int
	BurstSize         int
}

type APIKeyRecord struct {
	Key       string
	UserID    string
	Scopes    []string
	Enabled   bool
	ExpiresAt time.Time
	RateLimit APIKeyRateLimit
}
type APIKeyLookup interface {
	FindByKey(ctx context.Context, key string) (*APIKeyRecord, error)
}

type apiKeyContextKey struct{}

func Auth(lookup APIKeyLookup, log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := extractAPIKey(r)
			if key == "" {
				writeJSON(w, http.StatusUnauthorized, `{"error":"missing api key"}`)
				return
			}

			record, err := lookup.FindByKey(r.Context(), key)
			if err != nil {
				if errors.Is(err, ErrKeyNotFound) {
					writeJSON(w, http.StatusUnauthorized, `{"error":"invalid api key"}`)
					return
				}
				log.Error().Err(err).Msg("api key lookup failed")
				writeJSON(w, http.StatusInternalServerError, `{"error":"internal server error"}`)
				return
			}

			if !record.Enabled {
				writeJSON(w, http.StatusForbidden, `{"error":"api key disabled"}`)
				return
			}

			if !record.ExpiresAt.IsZero() && record.ExpiresAt.Before(time.Now()) {
				writeJSON(w, http.StatusForbidden, `{"error":"api key expired"}`)
				return
			}

			ctx := context.WithValue(r.Context(), apiKeyContextKey{}, record)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetAPIKey(ctx context.Context) *APIKeyRecord {
	record, ok := ctx.Value(apiKeyContextKey{}).(*APIKeyRecord)
	if !ok {
		return nil
	}
	return record
}

func extractAPIKey(r *http.Request) string {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}

	if key := r.URL.Query().Get("api_key"); key != "" {
		return key
	}

	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(body))
}
