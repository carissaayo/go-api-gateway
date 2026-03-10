package storage

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type RequestLog struct {
	Timestamp  time.Time         `bson:"timestamp"`
	Method     string            `bson:"method"`
	Path       string            `bson:"path"`
	StatusCode int               `bson:"status_code"`
	Duration   time.Duration     `bson:"duration_ms"`
	APIKey     string            `bson:"api_key"`
	RequestID  string            `bson:"request_id"`
	Backend    string            `bson:"backend"`
	Metadata   map[string]string `bson:"metadata"`
}

type AnalyticsRepository struct {
	collection *mongo.Collection
	logCh      chan RequestLog
	log        zerolog.Logger
}

func NewAnalyticsRepository(db *MongoDB, log zerolog.Logger, bufferSize int) *AnalyticsRepository {
	repo := &AnalyticsRepository{
		collection: db.Collection("request_logs"),
		logCh:      make(chan RequestLog, bufferSize),
		log:        log,
	}

	go repo.worker()

	return repo
}

func (r *AnalyticsRepository) Record(entry RequestLog) {
	select {
	case r.logCh <- entry:
	default:
		r.log.Warn().Msg("analytics buffer full, dropping log entry")
	}
}

func (r *AnalyticsRepository) worker() {
	batch := make([]interface{}, 0, 100)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case entry, ok := <-r.logCh:
			if !ok {
				if len(batch) > 0 {
					r.flush(batch)
				}
				return
			}
			batch = append(batch, entry)
			if len(batch) >= 100 {
				r.flush(batch)
				batch = make([]interface{}, 0, 100)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				r.flush(batch)
				batch = make([]interface{}, 0, 100)
			}
		}
	}
}

func (r *AnalyticsRepository) flush(batch []interface{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.InsertMany().SetOrdered(false)
	_, err := r.collection.InsertMany(ctx, batch, opts)
	if err != nil {
		r.log.Error().Err(err).Int("count", len(batch)).Msg("failed to flush analytics")
	}
}

func (r *AnalyticsRepository) Close() {
	close(r.logCh)
}
