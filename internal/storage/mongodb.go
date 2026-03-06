package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type MongoDB struct {
	client   *mongo.Client
	database *mongo.Database
}

func NewMongoDB(ctx context.Context, uri string, dbName string) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongodb connect: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("mongodb ping: %w", err)
	}

	return &MongoDB{
		client:   client,
		database: client.Database(dbName),
	}, nil
}

func (m *MongoDB) Collection(name string) *mongo.Collection {
	return m.database.Collection(name)
}

func (m *MongoDB) Close(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}

func (m *MongoDB) Ping(ctx context.Context) error {
	return m.client.Ping(ctx, readpref.Primary())
}
