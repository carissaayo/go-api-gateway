package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"

	"github.com/rs/zerolog"
)

type Backend struct {
	Name   string
	URL    *url.URL
	Alive  bool
	Weight int
}

type ReverseProxy struct {
	backends []*Backend
	current  atomic.Uint64
	log      zerolog.Logger
}

func New(log zerolog.Logger) *ReverseProxy {
	return &ReverseProxy{
		log: log,
	}
}

func (rp *ReverseProxy) AddBackend(name string, rawURL string, weight int) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse backend url: %w", err)
	}

	rp.backends = append(rp.backends, &Backend{
		Name:   name,
		URL:    parsed,
		Alive:  true,
		Weight: weight,
	})

	rp.log.Info().Str("name", name).Str("url", rawURL).Msg("backend registered")
	return nil
}

func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend := rp.nextBackend()
	if backend == nil {
		http.Error(w, `{"error":"no backends available"}`, http.StatusServiceUnavailable)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = backend.URL.Scheme
			req.URL.Host = backend.URL.Host
			req.Host = backend.URL.Host
			req.Header.Set("X-Forwarded-Host", r.Host)
			req.Header.Set("X-Forwarded-For", r.RemoteAddr)
			req.Header.Set("X-Gateway-Backend", backend.Name)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			rp.log.Error().Err(err).Str("backend", backend.Name).Msg("proxy error")
			http.Error(w, `{"error":"bad gateway"}`, http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

func (rp *ReverseProxy) nextBackend() *Backend {
	backends := rp.aliveBackends()
	if len(backends) == 0 {
		return nil
	}

	idx := rp.current.Add(1)
	return backends[idx%uint64(len(backends))]
}

func (rp *ReverseProxy) aliveBackends() []*Backend {
	var alive []*Backend
	for _, b := range rp.backends {
		if b.Alive {
			alive = append(alive, b)
		}
	}
	return alive
}
