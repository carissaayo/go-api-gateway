package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var ErrKeyNotFound = errors.New("api key not found")

type RateLimit struct {
	Algorithm         string `bson:"algorithm"`
	RequestsPerSecond int    `bson:"requests_per_second"`
	BurstSize         int    `bson:"burst_size"`
	ConcurrentLimit   int    `bson:"concurrent_limit"`
}

type APIKey struct {
	Key       string    `bson:"api_key"`
	UserID    string    `bson:"user_id"`
	Name      string    `bson:"name"`
	Scopes    []string  `bson:"scopes"`
	RateLimit RateLimit `bson:"rate_limit"`
	CreatedAt time.Time `bson:"created_at"`
	ExpiresAt time.Time `bson:"expires_at"`
	Enabled   bool      `bson:"enabled"`
}

type APIKeyRepository struct {
	collection *mongo.Collection
}

func NewAPIKeyRepository(db *MongoDB) *APIKeyRepository {
	return &APIKeyRepository{
		collection: db.Collection("api_keys"),
	}
}

func (r *APIKeyRepository) FindByKey(ctx context.Context, key string) (*APIKey, error) {
	var apiKey APIKey

	err := r.collection.FindOne(ctx, bson.M{"api_key": key}).Decode(&apiKey)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("find api key: %w", err)
	}

	return &apiKey, nil
}

func (r *APIKeyRepository) IsValid(key *APIKey) bool {
	if !key.Enabled {
		return false
	}
	if !key.ExpiresAt.IsZero() && key.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}
