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

	// Connect to MongoDB
	db, err := storage.NewMongoDB(context.Background(), cfg.MongoDB.URI, cfg.MongoDB.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to mongodb")
	}
	defer db.Close(context.Background())

	log.Info().Msg("connected to mongodb")

	apiKeyRepo := storage.NewAPIKeyRepository(db)

	gw := gateway.New(cfg, log, apiKeyRepo)

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := gw.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("shutdown error")
	}

	log.Info().Msg("gateway stopped gracefully")
}
