package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"

	"github.com/carissaayo/go-api-gateway/internal/circuitbreaker"
)

type Backend struct {
	Name           string
	URL            *url.URL
	Alive          bool
	Weight         int
	CircuitBreaker *circuitbreaker.CircuitBreaker
}

type ReverseProxy struct {
	backends []*Backend
	current  atomic.Uint64
	log      zerolog.Logger
	mu       sync.RWMutex
}

func New(log zerolog.Logger) *ReverseProxy {
	return &ReverseProxy{
		log: log,
	}
}

func (rp *ReverseProxy) AddBackend(name string, rawURL string, weight int, cbConfig circuitbreaker.Config) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse backend url: %w", err)
	}
	rp.mu.Lock()
	rp.backends = append(rp.backends, &Backend{
		Name:           name,
		URL:            parsed,
		Alive:          true,
		Weight:         weight,
		CircuitBreaker: circuitbreaker.New(cbConfig),
	})

	rp.mu.Unlock()
	rp.log.Info().Str("name", name).Str("url", rawURL).Msg("backend registered")
	return nil
}

func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend := rp.nextBackend()
	if backend == nil {
		http.Error(w, `{"error":"no backends available"}`, http.StatusServiceUnavailable)
		return
	}

	if !backend.CircuitBreaker.Allow() {
		rp.log.Warn().Str("backend", backend.Name).Str("state", backend.CircuitBreaker.State().String()).Msg("circuit breaker rejected request")
		http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
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

	wrapped := &statusCapture{ResponseWriter: w, statusCode: http.StatusOK}
	proxy.ServeHTTP(wrapped, r)

	if wrapped.statusCode >= 500 {
		backend.CircuitBreaker.RecordFailure()
	} else {
		backend.CircuitBreaker.RecordSuccess()
	}
}

type statusCapture struct {
	http.ResponseWriter
	statusCode int
}

func (sc *statusCapture) WriteHeader(code int) {
	sc.statusCode = code
	sc.ResponseWriter.WriteHeader(code)
}

func (rp *ReverseProxy) nextBackend() *Backend {
	rp.mu.RLock()
	backends := rp.aliveBackends()
	rp.mu.RUnlock()
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
