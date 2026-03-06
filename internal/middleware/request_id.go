package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

var ErrKeyNotFound = errors.New("api key not found")

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetRequestID(ctx context.Context) string {
	id, ok := ctx.Value(RequestIDKey).(string)
	if !ok {
		return ""
	}
	return id
}
