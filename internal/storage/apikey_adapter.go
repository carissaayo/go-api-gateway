package storage

import (
	"context"

	"github.com/carissaayo/go-api-gateway/internal/middleware"
)

type APIKeyAdapter struct {
	repo *APIKeyRepository
}

func NewAPIKeyAdapter(repo *APIKeyRepository) *APIKeyAdapter {
	return &APIKeyAdapter{repo: repo}
}

func (a *APIKeyAdapter) FindByKey(ctx context.Context, key string) (*middleware.APIKeyRecord, error) {
	apiKey, err := a.repo.FindByKey(ctx, key)
	if err != nil {
		if err == ErrKeyNotFound {
			return nil, middleware.ErrKeyNotFound
		}
		return nil, err
	}

	return &middleware.APIKeyRecord{
		Key:       apiKey.Key,
		UserID:    apiKey.UserID,
		Scopes:    apiKey.Scopes,
		Enabled:   apiKey.Enabled,
		ExpiresAt: apiKey.ExpiresAt,
		RateLimit: middleware.APIKeyRateLimit{
			RequestsPerSecond: apiKey.RateLimit.RequestsPerSecond,
			BurstSize:         apiKey.RateLimit.BurstSize,
		},
	}, nil
}
