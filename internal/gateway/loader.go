package gateway

import (
	"context"
	"time"

	"github.com/carissaayo/go-api-gateway/internal/circuitbreaker"
	"github.com/carissaayo/go-api-gateway/internal/storage"
)

func (gw *Gateway) LoadBackends(ctx context.Context, repo *storage.BackendRepository) error {
	backends, err := repo.FindAll(ctx)
	if err != nil {
		return err
	}

	for _, b := range backends {
		cbConfig := circuitbreaker.Config{
			MaxRequests:      b.CircuitBreaker.MaxRequests,
			Interval:         time.Duration(b.CircuitBreaker.Timeout) * time.Second,
			Timeout:          time.Duration(b.CircuitBreaker.Timeout) * time.Second,
			ErrorThreshold:   b.CircuitBreaker.ErrorThreshold,
			SuccessThreshold: b.CircuitBreaker.SuccessThreshold,
		}

		if cbConfig.MaxRequests == 0 {
			cbConfig = circuitbreaker.DefaultConfig()
		}

		if err := gw.proxy.AddBackend(b.Name, b.URL, b.Weight, cbConfig); err != nil {
			gw.log.Error().Err(err).Str("backend", b.Name).Msg("failed to register backend")
			continue
		}
	}

	gw.log.Info().Int("count", len(backends)).Msg("backends loaded from database")
	return nil
}

func (gw *Gateway) WatchBackends(ctx context.Context, repo *storage.BackendRepository) {
	for {
		err := gw.watchBackendsOnce(ctx, repo)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			gw.log.Error().Err(err).Msg("change stream error, reconnecting in 5s")
			time.Sleep(5 * time.Second)
		}
	}
}

func (gw *Gateway) watchBackendsOnce(ctx context.Context, repo *storage.BackendRepository) error {
	stream, err := repo.Watch(ctx)
	if err != nil {
		return err
	}
	defer stream.Close(ctx)

	gw.log.Info().Msg("watching for backend changes")

	for stream.Next(ctx) {
		gw.log.Info().Msg("backend config changed, reloading")

		if err := gw.reloadBackends(ctx, repo); err != nil {
			gw.log.Error().Err(err).Msg("failed to reload backends")
		}
	}

	return stream.Err()
}

func (gw *Gateway) reloadBackends(ctx context.Context, repo *storage.BackendRepository) error {
	backends, err := repo.FindAll(ctx)
	if err != nil {
		return err
	}

	gw.proxy.ClearBackends()

	for _, b := range backends {
		cbConfig := circuitbreaker.Config{
			MaxRequests:      b.CircuitBreaker.MaxRequests,
			Interval:         time.Duration(b.CircuitBreaker.Timeout) * time.Second,
			Timeout:          time.Duration(b.CircuitBreaker.Timeout) * time.Second,
			ErrorThreshold:   b.CircuitBreaker.ErrorThreshold,
			SuccessThreshold: b.CircuitBreaker.SuccessThreshold,
		}

		if cbConfig.MaxRequests == 0 {
			cbConfig = circuitbreaker.DefaultConfig()
		}

		if err := gw.proxy.AddBackend(b.Name, b.URL, b.Weight, cbConfig); err != nil {
			gw.log.Error().Err(err).Str("backend", b.Name).Msg("failed to register backend on reload")
			continue
		}
	}

	gw.log.Info().Int("count", len(backends)).Msg("backends reloaded")
	return nil
}
