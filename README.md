# API Gateway

An enterprise-grade reverse proxy built with Go, MongoDB, and Redis. Sits between clients and backend microservices providing authentication, rate limiting, circuit breaking, request transformation, and analytics.

---

## Table of Contents

- [Features](#features)
- [Tech Stack](#tech-stack)
- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Project Structure](#project-structure)
- [API Reference](#api-reference)
- [Rate Limiting](#rate-limiting)
- [Circuit Breaker](#circuit-breaker)
- [MongoDB Schema](#mongodb-schema)
- [Monitoring](#monitoring)
- [Future Improvements](#future-improvements)

---

## Features

- **Authentication** — API key validation via header (`X-API-Key`), query parameter, or `Authorization: Bearer` header
- **Rate Limiting** — Token bucket algorithm with configurable rate and burst, implemented as an atomic Redis Lua script
- **Circuit Breaker** — Per-backend state machine (Closed → Open → Half-Open) with configurable error threshold, timeout, and success threshold
- **Request Transformation** — Per-route header injection/stripping, path prefix rewriting
- **Reverse Proxy** — `httputil.ReverseProxy` with round-robin load balancing across multiple backends
- **Analytics** — Async request logging to MongoDB via buffered channels with batch writes
- **Observability** — Prometheus metrics (counters, histograms, gauges) + Zerolog structured JSON logging
- **Dynamic Config** — Backends loaded from MongoDB at startup with hot-reload via change streams (requires replica set)
- **Graceful Shutdown** — OS signal handling with in-flight request draining
- **Panic Recovery** — Middleware that catches panics and returns 500 instead of crashing

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25 |
| HTTP Router | Chi v5 |
| Database | MongoDB 7+ |
| Cache / Rate Limiting | Redis 7+ |
| Metrics | Prometheus |
| Logging | Zerolog |

---

## Prerequisites

- Go 1.21+
- MongoDB 7+ (local or Docker)
- Redis 7+ (local via Memurai on Windows, or Docker)

---

## Getting Started

### 1. Clone and install dependencies

```bash
git clone https://github.com/carissaayo/go-api-gateway.git
cd go-api-gateway
go mod download
```

### 2. Start infrastructure

Make sure MongoDB and Redis are running locally. On Windows, Redis can be run via [Memurai](https://www.memurai.com/).

### 3. Seed test data

Using MongoDB Compass (or `mongosh`), connect to `mongodb://localhost:27017` and create the following in the `api_gateway` database.

**Collection: `api_keys`**

```json
{
  "api_key": "gw_test_key",
  "user_id": "user_1",
  "name": "Test Key",
  "scopes": ["read", "write"],
  "rate_limit": {
    "algorithm": "token_bucket",
    "requests_per_second": 100,
    "burst_size": 200,
    "concurrent_limit": 10
  },
  "created_at": { "$date": "2026-03-06T00:00:00Z" },
  "expires_at": { "$date": "2027-01-01T00:00:00Z" },
  "enabled": true
}
```

**Collection: `backends`**

```json
{
  "name": "test-service",
  "url": "http://localhost:3001",
  "weight": 1,
  "enabled": true,
  "health_check": {
    "path": "/health",
    "interval": 30,
    "timeout": 5
  },
  "circuit_breaker": {
    "error_threshold": 0.5,
    "timeout": 30,
    "max_requests": 10,
    "success_threshold": 5
  }
}
```

### 4. Run a backend service

Start any HTTP server on port 3001 to act as the upstream backend:

```bash
npx http-server -p 3001
```

### 5. Run the gateway

```bash
cp .env.example .env
go run cmd/gateway/main.go
```

Gateway starts on `http://localhost:8080`.

### 6. Test it

```bash
# Health check (no auth)
curl.exe http://localhost:8080/health

# Authenticated request (proxied to backend)
curl.exe -H "X-API-Key: gw_test_key" http://localhost:8080/api/test

# Without key (returns 401)
curl.exe http://localhost:8080/api/test
```

---

## Configuration

All config is loaded from environment variables (see `.env.example`):

```env
# Server
SERVER_PORT=8080
SERVER_READ_TIMEOUT=15s
SERVER_WRITE_TIMEOUT=15s
SERVER_IDLE_TIMEOUT=60s

# MongoDB
MONGODB_URI=mongodb://localhost:27017
MONGODB_DB=api_gateway

# Redis
REDIS_URL=redis://localhost:6379

# Logging
LOG_LEVEL=info        # debug | info | warn | error
LOG_FORMAT=json       # json | pretty
```

---

## Project Structure

```
go-api-gateway/
├── cmd/
│   └── gateway/
│       └── main.go                    # Entry point, dependency wiring, graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go                 # Env-based config loading with validation
│   ├── gateway/
│   │   ├── gateway.go                # Core gateway struct, middleware + route setup
│   │   ├── backend.go                # Backend registration helper
│   │   └── loader.go                 # Dynamic backend loading + change stream watcher
│   ├── middleware/
│   │   ├── auth.go                   # API key validation middleware
│   │   ├── ratelimit.go              # Rate limit middleware
│   │   ├── transform.go              # Request/response header transformation
│   │   ├── analytics.go              # Prometheus metrics + async MongoDB logging
│   │   ├── logging.go                # Structured request logging
│   │   ├── request_id.go             # UUID request ID generation
│   │   └── recovery.go               # Panic recovery
│   ├── ratelimit/
│   │   ├── token_bucket.go           # Token bucket algorithm (Go + Redis)
│   │   └── lua/
│   │       └── token_bucket.lua      # Atomic Redis Lua script
│   ├── circuitbreaker/
│   │   └── circuitbreaker.go         # State machine (Closed/Open/Half-Open)
│   ├── storage/
│   │   ├── mongodb.go                # MongoDB client wrapper
│   │   ├── apikey.go                 # API key repository
│   │   ├── apikey_adapter.go         # API key adapter (storage → middleware types)
│   │   ├── analytics.go              # Async analytics repository (buffered channel + batch writes)
│   │   ├── analytics_adapter.go      # Analytics adapter (middleware → storage types)
│   │   └── backend.go                # Backend config repository
│   ├── redis/
│   │   └── client.go                 # Redis client wrapper
│   ├── proxy/
│   │   └── reverse_proxy.go          # Reverse proxy with round-robin + circuit breaker
│   ├── logger/
│   │   └── logger.go                 # Zerolog configuration
│   └── metrics/
│       └── metrics.go                # Prometheus metric definitions
├── .env.example
├── .gitignore
└── README.md
```

---

## API Reference

### Request Headers

| Header | Required | Description |
|--------|----------|-------------|
| `X-API-Key` | Yes* | API key for authentication |
| `Authorization` | Yes* | `Bearer <key>` as alternative to X-API-Key |

*API key can also be passed as `?api_key=` query parameter.

### Response Headers

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Configured burst size |
| `X-RateLimit-Remaining` | Tokens remaining in current window |
| `Retry-After` | Seconds until quota resets (on 429) |
| `X-Request-ID` | Unique request ID for tracing |
| `X-Powered-By` | Gateway identifier (via response transform) |

### Status Codes

| Code | Meaning |
|------|---------|
| `401 Unauthorized` | Missing or invalid API key |
| `403 Forbidden` | Key disabled or expired |
| `429 Too Many Requests` | Rate limit exceeded |
| `502 Bad Gateway` | Backend unreachable |
| `503 Service Unavailable` | Circuit breaker is OPEN or no backends available |

### Health Endpoints

```
GET /health   → 200 OK  (liveness)
GET /ready    → 200 OK  (readiness)
GET /metrics  → Prometheus metrics exposition
```

---

## Rate Limiting

Rate limits are configured per API key in the `api_keys` MongoDB collection.

### Token Bucket

Tokens refill at a constant rate. Allows bursts up to `burst_size`. Implemented as an atomic Redis Lua script — no race conditions under concurrent load.

```json
"rate_limit": {
  "algorithm": "token_bucket",
  "requests_per_second": 100,
  "burst_size": 200
}
```

The rate limiter **fails open** — if Redis is unreachable, requests are allowed through rather than blocking all traffic.

### Redis Key Pattern

```
rate_limit:token_bucket:{api_key}
```

---

## Circuit Breaker

Per-backend state machine protecting against cascading failures.

```
CLOSED ──(error rate exceeds threshold)──→ OPEN
  ↑                                          │
  │                                     (timeout expires)
  │                                          │
  │                                          ▼
  └──(enough consecutive successes)──── HALF-OPEN
                                          │
                                (test request fails)
                                          │
                                          ▼
                                        OPEN
```

| State | Behaviour |
|-------|-----------|
| **CLOSED** | Normal operation. Tracks error rate over a rolling window. |
| **OPEN** | Returns 503 immediately. No requests forwarded to backend. |
| **HALF-OPEN** | Allows `max_requests` test requests through. |

### Configuration (per backend in MongoDB)

```json
"circuit_breaker": {
  "max_requests": 10,
  "timeout": 30,
  "error_threshold": 0.5,
  "success_threshold": 5
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `max_requests` | 10 | Max test requests in HALF-OPEN |
| `timeout` | 30s | Time to stay OPEN before testing |
| `error_threshold` | 0.5 | Error rate (0–1) that trips the breaker |
| `success_threshold` | 5 | Consecutive successes to close from HALF-OPEN |

---

## MongoDB Schema

### `api_keys`

```json
{
  "api_key": "gw_test_key",
  "user_id": "user_1",
  "name": "Test Key",
  "scopes": ["read", "write"],
  "rate_limit": {
    "algorithm": "token_bucket",
    "requests_per_second": 100,
    "burst_size": 200
  },
  "created_at": "2026-03-01T00:00:00Z",
  "expires_at": "2027-03-01T00:00:00Z",
  "enabled": true
}
```

### `backends`

```json
{
  "name": "user-service",
  "url": "http://user-service:3001",
  "weight": 1,
  "enabled": true,
  "health_check": { "path": "/health", "interval": 30, "timeout": 5 },
  "circuit_breaker": { "error_threshold": 0.5, "timeout": 30, "max_requests": 10, "success_threshold": 5 }
}
```

### `request_logs`

Written asynchronously by the analytics pipeline. Documents appear within 5 seconds of request completion.

```json
{
  "timestamp": "2026-03-06T15:52:09Z",
  "method": "GET",
  "path": "/api/test",
  "status_code": 200,
  "duration_ms": 20,
  "api_key": "gw_test_key",
  "request_id": "fc4d92e1-b50f-45af-9e3d-1d5cd18186d3"
}
```

---

## Monitoring

### Prometheus Metrics

```bash
curl http://localhost:8080/metrics
```

Key metrics exposed:

| Metric | Type | Description |
|--------|------|-------------|
| `gateway_requests_total` | Counter | Total requests by method, path, status |
| `gateway_request_duration_seconds` | Histogram | Request latency distribution |
| `gateway_rate_limit_hits_total` | Counter | Total rate-limited requests |
| `gateway_circuit_breaker_state` | Gauge | Circuit breaker state per backend |

Example PromQL queries:

```promql
# Request rate
rate(gateway_requests_total[5m])

# Error rate
rate(gateway_requests_total{status=~"5.."}[5m]) / rate(gateway_requests_total[5m])

# p95 latency
histogram_quantile(0.95, rate(gateway_request_duration_seconds_bucket[5m]))

# Rate limit hit rate
rate(gateway_rate_limit_hits_total[5m])
```

### MongoDB Analytics

```javascript
// Top 10 endpoints by request volume (last 24h)
db.request_logs.aggregate([
  { $match: { timestamp: { $gte: new Date(Date.now() - 86400000) } } },
  { $group: { _id: "$path", count: { $sum: 1 }, avg_latency: { $avg: "$duration_ms" } } },
  { $sort: { count: -1 } },
  { $limit: 10 }
])
```

---

## Future Improvements

- [ ] JWT and OAuth2 token introspection authentication
- [ ] Sliding window and concurrent request rate limiting algorithms
- [ ] Backend health checks (active polling)
- [ ] Dockerfile and Docker Compose setup
- [ ] Kubernetes deployment manifests
- [ ] k6 load testing scripts
- [ ] Makefile for common operations
- [ ] Seed data script (`scripts/seed_data.sh`)
- [ ] Unit tests with miniredis and testcontainers-go
- [ ] Grafana dashboard templates
- [ ] MongoDB TTL index on `request_logs` (30-day retention)
- [ ] Per-route transform configuration loaded from MongoDB

---

## License

MIT
