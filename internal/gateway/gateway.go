package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/carissaayo/go-api-gateway/internal/config"
)

type Gateway struct {
	config *config.Config
	router chi.Router
	server *http.Server
}

func New(cfg *config.Config) *Gateway {
	r := chi.NewRouter()

	gw := &Gateway{
		config: cfg,
		router: r,
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
			Handler:      r,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		},
	}

	gw.setupRoutes()

	return gw
}

func (gw *Gateway) setupRoutes() {
	gw.router.Get("/health", gw.healthCheck)
	gw.router.Get("/ready", gw.readinessCheck)
}

func (gw *Gateway) Start() error {
	fmt.Printf("Gateway listening on %s\n", gw.server.Addr)
	return gw.server.ListenAndServe()
}

func (gw *Gateway) Shutdown(ctx context.Context) error {
	return gw.server.Shutdown(ctx)
}

func (gw *Gateway) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (gw *Gateway) readinessCheck(w http.ResponseWriter, r *http.Request) {
	// Later: check MongoDB + Redis connectivity
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}
