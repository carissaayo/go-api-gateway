package storage

import (
	"github.com/carissaayo/go-api-gateway/internal/middleware"
)

type AnalyticsAdapter struct {
	repo *AnalyticsRepository
}

func NewAnalyticsAdapter(repo *AnalyticsRepository) *AnalyticsAdapter {
	return &AnalyticsAdapter{repo: repo}
}

func (a *AnalyticsAdapter) Record(entry middleware.AnalyticsEntry) {
	a.repo.Record(RequestLog{
		Timestamp:  entry.Timestamp,
		Method:     entry.Method,
		Path:       entry.Path,
		StatusCode: entry.StatusCode,
		Duration:   entry.Duration,
		APIKey:     entry.APIKey,
		RequestID:  entry.RequestID,
	})
}
