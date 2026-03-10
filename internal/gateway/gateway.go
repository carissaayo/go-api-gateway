package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"github.com/carissaayo/go-api-gateway/internal/config"
	"github.com/carissaayo/go-api-gateway/internal/middleware"
	"github.com/carissaayo/go-api-gateway/internal/proxy"
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
	proxy         *proxy.ReverseProxy
}

func New(cfg *config.Config, log zerolog.Logger, apiKeyRepo *storage.APIKeyRepository, rateLimiter *ratelimit.TokenBucket) *Gateway {
	r := chi.NewRouter()
	adapter := storage.NewAPIKeyAdapter(apiKeyRepo)
	rp := proxy.New(log)
	gw := &Gateway{
		config:        cfg,
		router:        r,
		log:           log,
		apiKeyRepo:    apiKeyRepo,
		apiKeyAdapter: adapter,
		rateLimiter:   rateLimiter,
		proxy:         rp,
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
	gw.router.Handle("/metrics", promhttp.Handler())
	gw.router.Group(func(r chi.Router) {
		r.Use(middleware.Auth(gw.apiKeyAdapter, gw.log))
		r.Use(middleware.RateLimit(gw.rateLimiter, gw.log))
		r.Use(middleware.Transform(middleware.TransformConfig{
			Request: middleware.RequestTransform{
				AddHeaders: map[string]string{
					"X-Gateway-Version": "v1",
				},
				RemoveHeaders: []string{"X-Internal-Debug"},
				StripPrefix:   "/api",
			},
			Response: middleware.ResponseTransform{
				AddHeaders: map[string]string{
					"X-Powered-By": "go-api-gateway",
				},
				RemoveHeaders: []string{"X-Internal-Token", "Server"},
			},
		}))
		r.Handle("/api/*", gw.proxy)
	})
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
