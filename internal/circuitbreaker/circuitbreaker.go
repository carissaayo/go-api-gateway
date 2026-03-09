package circuitbreaker

import (
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

type Config struct {
	MaxRequests      int
	Interval         time.Duration
	Timeout          time.Duration
	ErrorThreshold   float64
	SuccessThreshold int
}

func DefaultConfig() Config {
	return Config{
		MaxRequests:      10,
		Interval:         60 * time.Second,
		Timeout:          30 * time.Second,
		ErrorThreshold:   0.5,
		SuccessThreshold: 5,
	}
}

type counts struct {
	requests    int
	successes   int
	failures    int
	consecutive int
}

func (c *counts) reset() {
	c.requests = 0
	c.successes = 0
	c.failures = 0
	c.consecutive = 0
}

func (c *counts) errorRate() float64 {
	if c.requests == 0 {
		return 0
	}
	return float64(c.failures) / float64(c.requests)
}

type CircuitBreaker struct {
	mu          sync.RWMutex
	state       State
	config      Config
	counts      counts
	lastFailure time.Time
	openedAt    time.Time
}

func New(cfg Config) *CircuitBreaker {
	return &CircuitBreaker{
		state:  StateClosed,
		config: cfg,
	}
}

func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == StateOpen && time.Since(cb.openedAt) > cb.config.Timeout {
		return StateHalfOpen
	}

	return cb.state
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.openedAt) > cb.config.Timeout {
			cb.toHalfOpen()
			return true
		}
		return false
	case StateHalfOpen:
		return cb.counts.requests < cb.config.MaxRequests
	default:
		return false
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.counts.requests++
	cb.counts.successes++
	cb.counts.consecutive++

	if cb.state == StateHalfOpen && cb.counts.consecutive >= cb.config.SuccessThreshold {
		cb.toClosed()
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.counts.requests++
	cb.counts.failures++
	cb.counts.consecutive = 0
	cb.lastFailure = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.counts.requests >= cb.config.MaxRequests && cb.counts.errorRate() >= cb.config.ErrorThreshold {
			cb.toOpen()
		}
	case StateHalfOpen:
		cb.toOpen()
	}
}

func (cb *CircuitBreaker) toClosed() {
	cb.state = StateClosed
	cb.counts.reset()
}

func (cb *CircuitBreaker) toOpen() {
	cb.state = StateOpen
	cb.openedAt = time.Now()
	cb.counts.reset()
}

func (cb *CircuitBreaker) toHalfOpen() {
	cb.state = StateHalfOpen
	cb.counts.reset()
}
