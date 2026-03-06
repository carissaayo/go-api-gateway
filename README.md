# API Gateway

An enterprise-grade reverse proxy built with Go, MongoDB, and Redis. Sits between clients and backend microservices providing authentication, rate limiting, circuit breaking, request transformation, and analytics.

**Performance targets:** 50,000+ req/s · <5ms p95 gateway overhead · 99.9% uptime

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
- [Testing](#testing)
- [Load Testing](#load-testing)
- [Deployment](#deployment)
- [Monitoring](#monitoring)

---

## Features

- **Authentication** — API key (header/query param), JWT, OAuth2 token introspection
- **Rate Limiting** — Token bucket, sliding window, and concurrent request limiting via Redis Lua scripts
- **Circuit Breaker** — State machine (Closed → Open → Half-Open) with configurable thresholds
- **Request Transformation** — Per-route header injection/stripping, path rewriting
- **Reverse Proxy** — `httputil.ReverseProxy` with round-robin load balancing
- **Analytics** — MongoDB time-series request logs with 30-day TTL
- **Observability** — Prometheus metrics + Zerolog structured JSON logging + Grafana dashboards
- **Dynamic Config** — Routes and backends loaded from MongoDB with hot-reload via change streams

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.21+ |
| HTTP Router | Chi v5 |
| Database | MongoDB 7+ |
| Cache / Rate Limiting | Redis 7+ |
| Metrics | Prometheus |
| Logging | Zerolog |
| Testing | Testify + miniredis |
| Load Testing | k6 |

---

## Prerequisites

- Go 1.21+
- Docker & Docker Compose
- MongoDB 7+ (or use the provided Compose file)
- Redis 7+ (or use the provided Compose file)

---

## Getting Started

### 1. Clone and install dependencies

```bash
git clone https://github.com/yourusername/api-gateway.git
cd api-gateway
go mod download
```

### 2. Start infrastructure

```bash
docker-compose -f docker/docker-compose.yml up -d mongo redis
```

### 3. Seed test data

```bash
bash scripts/seed_data.sh
```

This creates a test API key (`gw_test_key`) and registers a sample backend.

### 4. Run the gateway

```bash
cp .env.example .env
go run cmd/gateway/main.go
```

Gateway starts on `http://localhost:8080`.

### 5. Test it

```bash
curl -H "X-API-Key: gw_test_key" http://localhost:8080/api/test
```

---

## Configuration

All config is loaded from environment variables (see `.env.example`):

```env
# Server
SERVER_PORT=8080

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
api-gateway/
├── cmd/
│   └── gateway/
│       └── main.go                 # Entry point, graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go              # Env-based config loading
│   ├── gateway/
│   │   ├── gateway.go             # Core gateway, middleware orchestration
│   │   ├── backend.go             # Backend registry, health checks
│   │   └── route.go               # Route definitions
│   ├── middleware/
│   │   ├── auth.go                # API key / JWT validation
│   │   ├── ratelimit.go           # Rate limit middleware
│   │   ├── circuitbreaker.go      # Circuit breaker middleware
│   │   ├── transform.go           # Request/response transformation
│   │   ├── analytics.go           # Async request logging
│   │   └── recovery.go            # Panic recovery
│   ├── ratelimit/
│   │   ├── token_bucket.go        # Token bucket algorithm
│   │   ├── sliding_window.go      # Sliding window algorithm
│   │   ├── concurrent.go          # Concurrent request limiter
│   │   └── lua/
│   │       └── token_bucket.lua   # Atomic Redis Lua script
│   ├── circuitbreaker/
│   │   └── circuitbreaker.go      # State machine implementation
│   ├── storage/
│   │   ├── mongodb.go             # MongoDB client
│   │   ├── apikey.go              # API key repository
│   │   └── analytics.go           # Analytics repository
│   ├── redis/
│   │   └── client.go              # Redis client wrapper
│   ├── proxy/
│   │   └── reverse_proxy.go       # Custom reverse proxy
│   └── metrics/
│       └── metrics.go             # Prometheus metric definitions
├── scripts/
│   ├── load_test.js               # k6 load test
│   └── seed_data.sh               # Seed MongoDB with test data
├── docker/
│   ├── Dockerfile
│   └── docker-compose.yml
├── deployments/
│   └── kubernetes/                # K8s manifests
├── .env.example
├── Makefile
└── README.md
```

---

## API Reference

### Request Headers

| Header | Required | Description |
|--------|----------|-------------|
| `X-API-Key` | Yes* | API key for authentication |
| `Authorization` | Yes* | `Bearer <jwt>` for JWT auth |

*One of the two is required depending on route config.

### Response Headers

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Configured requests per second |
| `X-RateLimit-Remaining` | Tokens remaining in current window |
| `Retry-After` | Seconds until quota resets (on 429) |
| `X-Request-ID` | Unique request ID for tracing |

### Status Codes

| Code | Meaning |
|------|---------|
| `401 Unauthorized` | Missing or invalid API key |
| `403 Forbidden` | Key disabled or expired |
| `429 Too Many Requests` | Rate limit exceeded |
| `503 Service Unavailable` | Circuit breaker is OPEN |

### Health Endpoints

```
GET /health   → 200 OK  (liveness)
GET /ready    → 200 OK  (readiness — checks MongoDB + Redis connectivity)
GET /metrics  → Prometheus metrics exposition
```

---

## Rate Limiting

Three algorithms are available per API key, configured in the `api_keys` MongoDB collection.

### Token Bucket (default)

Tokens refill at a constant rate. Allows bursts up to `burst_size`.

```json
"rate_limit": {
  "algorithm": "token_bucket",
  "requests_per_second": 100,
  "burst_size": 200
}
```

### Sliding Window

Counts exact requests within a rolling time window. No burst allowance.

```json
"rate_limit": {
  "algorithm": "sliding_window",
  "requests_per_second": 100
}
```

### Concurrent Limiter

Limits the number of simultaneously in-flight requests per key.

```json
"rate_limit": {
  "concurrent_limit": 10
}
```

### Redis Key Patterns

```
rate_limit:token_bucket:{api_key}
rate_limit:sliding_window:{api_key}
rate_limit:concurrent:{api_key}
```

---

## Circuit Breaker

Per-backend state machine protecting against cascading failures.

```
CLOSED → OPEN → HALF-OPEN → CLOSED
                          ↘ OPEN
```

| State | Behaviour |
|-------|-----------|
| **CLOSED** | Normal operation. Tracks error rate. |
| **OPEN** | Returns 503 immediately. No requests forwarded. |
| **HALF-OPEN** | Allows `max_requests` test requests through. |

### Configuration (per backend in MongoDB)

```json
"circuit_breaker": {
  "max_requests": 10,
  "interval": 60,
  "timeout": 30,
  "error_threshold": 0.5,
  "success_threshold": 5
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `max_requests` | 10 | Max test requests in HALF-OPEN |
| `interval` | 60s | Rolling window for error rate tracking |
| `timeout` | 30s | Time to stay OPEN before testing |
| `error_threshold` | 0.5 | Error rate (0–1) that trips the breaker |
| `success_threshold` | 5 | Consecutive successes to close from HALF-OPEN |

---

## MongoDB Schema

### `api_keys`

```json
{
  "api_key": "gw_abc123",
  "user_id": "user_123",
  "name": "Production Key",
  "scopes": ["read:users", "write:posts"],
  "rate_limit": {
    "algorithm": "token_bucket",
    "requests_per_second": 100,
    "burst_size": 200,
    "concurrent_limit": 10
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
  "health_check": { "path": "/health", "interval": 30, "timeout": 5 },
  "circuit_breaker": { "error_threshold": 0.5, "timeout": 30 },
  "weight": 1,
  "enabled": true
}
```

### `routes`

```json
{
  "path": "/api/users",
  "methods": ["GET", "POST"],
  "backend": "user-service",
  "strip_path": true,
  "middlewares": ["auth", "ratelimit", "circuitbreaker"],
  "transform": {
    "request": { "add_headers": { "X-Gateway": "v1" } },
    "response": { "remove_headers": ["X-Internal-Token"] }
  }
}
```

---

## Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run specific package
go test ./internal/ratelimit/...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Unit tests use [miniredis](https://github.com/alicebob/miniredis) for Redis and [testcontainers-go](https://github.com/testcontainers/testcontainers-go) for MongoDB integration tests — no real infrastructure required.

---

## Load Testing

Requires [k6](https://k6.io/docs/getting-started/installation/).

```bash
k6 run scripts/load_test.js
```

Default scenario: ramp to 100 virtual users over 30s, hold for 1 minute, ramp down.

**Thresholds:**
- p95 request duration < 500ms
- Error rate < 1%

To test rate limiting specifically:

```bash
k6 run --vus 200 --duration 30s scripts/load_test.js
```

---

## Deployment

### Docker Compose (local / staging)

```bash
docker-compose -f docker/docker-compose.yml up --build
```

### Kubernetes (production)

```bash
# Create secrets
kubectl create secret generic gateway-secrets \
  --from-literal=mongodb-uri='mongodb://...' \
  --from-literal=redis-url='redis://...'

# Apply manifests
kubectl apply -f deployments/kubernetes/
```

The Kubernetes deployment runs 3 replicas with liveness (`/health`) and readiness (`/ready`) probes. Resources: 128Mi–512Mi memory, 250m–1000m CPU per pod.

---

## Monitoring

### Prometheus + Grafana

```bash
# Metrics endpoint
curl http://localhost:8080/metrics
```

Key metrics:

```promql
# Request rate
rate(gateway_requests_total[5m])

# Error rate
rate(gateway_requests_total{status=~"5.."}[5m]) / rate(gateway_requests_total[5m])

# p95 latency
histogram_quantile(0.95, rate(gateway_request_duration_seconds_bucket[5m]))

# Rate limit hit rate
rate(gateway_rate_limit_hits_total[5m])

# Circuit breakers currently open
sum(gateway_circuit_breaker_state == 1)
```

### MongoDB Analytics

```javascript
// Top 10 endpoints by request volume (last 24h)
db.request_logs.aggregate([
  { $match: { timestamp: { $gte: new Date(Date.now() - 86400000) } } },
  { $group: { _id: "$metadata.route", count: { $sum: 1 }, avg_latency: { $avg: "$latency_ms" } } },
  { $sort: { count: -1 } },
  { $limit: 10 }
])

// Error rate by backend
db.request_logs.aggregate([
  { $group: {
    _id: "$metadata.backend",
    total: { $sum: 1 },
    errors: { $sum: { $cond: [{ $gte: ["$metadata.status_code", 400] }, 1, 0] } }
  }},
  { $project: { error_rate: { $multiply: [{ $divide: ["$errors", "$total"] }, 100] } } }
])
```

---

## Makefile

```bash
make run          # Start gateway locally
make test         # Run test suite
make lint         # Run golangci-lint
make build        # Build binary to ./bin/gateway
make docker       # Build Docker image
make seed         # Seed MongoDB with test data
make load-test    # Run k6 load test
```

---

## License

MIT