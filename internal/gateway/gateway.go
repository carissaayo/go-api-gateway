package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/carissaayo/go-api-gateway/internal/config"
	"github.com/carissaayo/go-api-gateway/internal/middleware"
)

type Gateway struct {
	config *config.Config
	router chi.Router
	server *http.Server
	log    zerolog.Logger
}

func New(cfg *config.Config, log zerolog.Logger) *Gateway {
	r := chi.NewRouter()

	gw := &Gateway{
		config: cfg,
		router: r,
		log:    log,
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
