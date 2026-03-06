package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/carissaayo/go-api-gateway/internal/config"
	"github.com/carissaayo/go-api-gateway/internal/middleware"
	"github.com/carissaayo/go-api-gateway/internal/ratelimit"
	"github.com/carissaayo/go-api-gateway/internal/storage"
)

type Gateway struct {
	config        *config.Config
	router        chi.Router
	server        *http.Server
	log           zerolog.Logger
	apiKeyRepo    *storage.APIKeyRepository
	apiKeyAdapter *storage.APIKeyAdapter
	rateLimiter   *ratelimit.TokenBucket
}

func New(cfg *config.Config, log zerolog.Logger, apiKeyRepo *storage.APIKeyRepository, rateLimiter *ratelimit.TokenBucket) *Gateway {
	r := chi.NewRouter()
	adapter := storage.NewAPIKeyAdapter(apiKeyRepo)
	gw := &Gateway{
		config:        cfg,
		router:        r,
		log:           log,
		apiKeyRepo:    apiKeyRepo,
		apiKeyAdapter: adapter,
		rateLimiter:   rateLimiter,
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
			Handler:      r,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		},
	}

	gw.setupMiddleware()
	gw.setupRoutes()

	return gw
}

func (gw *Gateway) setupMiddleware() {
	gw.router.Use(middleware.RequestID)
	gw.router.Use(middleware.Logging(gw.log))
}

func (gw *Gateway) setupRoutes() {
	gw.router.Get("/health", gw.healthCheck)
	gw.router.Get("/ready", gw.readinessCheck)
	// Protected routes — auth middleware applied to this group only
	gw.router.Group(func(r chi.Router) {
		r.Use(middleware.Auth(gw.apiKeyAdapter, gw.log))
		r.Use(middleware.RateLimit(gw.rateLimiter, gw.log))
		r.Get("/api/*", func(w http.ResponseWriter, r *http.Request) {
			record := middleware.GetAPIKey(r.Context())
			writeJSON(w, http.StatusOK, `{"message":"authenticated","user":"`+record.UserID+`"}`)
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(body))
}

func (gw *Gateway) Start() error {
	gw.log.Info().Str("addr", gw.server.Addr).Msg("gateway starting")
	return gw.server.ListenAndServe()
}

func (gw *Gateway) Shutdown(ctx context.Context) error {
	gw.log.Info().Msg("gateway shutting down")
	return gw.server.Shutdown(ctx)
}

func (gw *Gateway) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (gw *Gateway) readinessCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}
