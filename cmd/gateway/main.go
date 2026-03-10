package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/carissaayo/go-api-gateway/internal/config"
	"github.com/carissaayo/go-api-gateway/internal/gateway"
	"github.com/carissaayo/go-api-gateway/internal/logger"
	"github.com/carissaayo/go-api-gateway/internal/ratelimit"
	redisclient "github.com/carissaayo/go-api-gateway/internal/redis"
	"github.com/carissaayo/go-api-gateway/internal/storage"
)

func main() {
	godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("failed to load config: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logger.New(cfg.Logging.Level, cfg.Logging.Format)

	db, err := storage.NewMongoDB(context.Background(), cfg.MongoDB.URI, cfg.MongoDB.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to mongodb")
	}
	defer db.Close(context.Background())
	log.Info().Msg("connected to mongodb")

	rc, err := redisclient.New(context.Background(), cfg.Redis.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to redis")
	}
	defer rc.Close()
	log.Info().Msg("connected to redis")

	apiKeyRepo := storage.NewAPIKeyRepository(db)
	backendRepo := storage.NewBackendRepository(db)
	rateLimiter := ratelimit.NewTokenBucket(rc.GetClient())

	gw := gateway.New(cfg, log, apiKeyRepo, rateLimiter)

	// Load backends from MongoDB
	if err := gw.LoadBackends(context.Background(), backendRepo); err != nil {
		log.Error().Err(err).Msg("failed to load backends from database")
	}

	// Watch for backend changes in background
	watchCtx, watchCancel := context.WithCancel(context.Background())
	defer watchCancel()
	go gw.WatchBackends(watchCtx, backendRepo)

	errCh := make(chan error, 1)
	go func() {
		if err := gw.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("received shutdown signal")
	case err := <-errCh:
		log.Fatal().Err(err).Msg("server error")
	}

	watchCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := gw.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("shutdown error")
	}

	log.Info().Msg("gateway stopped gracefully")
}
