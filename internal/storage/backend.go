package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type BackendConfig struct {
	Name           string               `bson:"name"`
	URL            string               `bson:"url"`
	Weight         int                  `bson:"weight"`
	Enabled        bool                 `bson:"enabled"`
	HealthCheck    HealthCheckConfig    `bson:"health_check"`
	CircuitBreaker CircuitBreakerConfig `bson:"circuit_breaker"`
}

type HealthCheckConfig struct {
	Path     string `bson:"path"`
	Interval int    `bson:"interval"`
	Timeout  int    `bson:"timeout"`
}

type CircuitBreakerConfig struct {
	ErrorThreshold   float64 `bson:"error_threshold"`
	Timeout          int     `bson:"timeout"`
	MaxRequests      int     `bson:"max_requests"`
	SuccessThreshold int     `bson:"success_threshold"`
}

type BackendRepository struct {
	collection *mongo.Collection
}

func NewBackendRepository(db *MongoDB) *BackendRepository {
	return &BackendRepository{
		collection: db.Collection("backends"),
	}
}

func (r *BackendRepository) FindAll(ctx context.Context) ([]BackendConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{"enabled": true})
	if err != nil {
		return nil, fmt.Errorf("find backends: %w", err)
	}
	defer cursor.Close(ctx)

	var backends []BackendConfig
	if err := cursor.All(ctx, &backends); err != nil {
		return nil, fmt.Errorf("decode backends: %w", err)
	}

	return backends, nil
}

func (r *BackendRepository) Watch(ctx context.Context) (*mongo.ChangeStream, error) {
	return r.collection.Watch(ctx, mongo.Pipeline{})
}
