package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/token_bucket.lua
var tokenBucketScript string

type TokenBucket struct {
	script *redis.Script
	rdb    *redis.Client
}

type Result struct {
	Allowed   bool
	Remaining int
	Limit     int
}

func NewTokenBucket(rdb *redis.Client) *TokenBucket {
	return &TokenBucket{
		script: redis.NewScript(tokenBucketScript),
		rdb:    rdb,
	}
}

func (tb *TokenBucket) Allow(ctx context.Context, key string, rate float64, burst int) (*Result, error) {
	now := float64(time.Now().UnixMicro()) / 1e6

	redisKey := fmt.Sprintf("rate_limit:token_bucket:%s", key)

	res, err := tb.script.Run(ctx, tb.rdb, []string{redisKey}, rate, burst, now).Int64Slice()
	if err != nil {
		return nil, fmt.Errorf("token bucket script: %w", err)
	}

	return &Result{
		Allowed:   res[0] == 1,
		Remaining: int(res[1]),
		Limit:     int(res[2]),
	}, nil
}
