package singleflight

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrInFlight = errors.New("request is already processing")

type RedisFlight struct {
	client    *redis.Client
	lockTTL   time.Duration
	resultTTL time.Duration
}

type Result struct {
	Value  string
	Replay bool
}

func NewRedisFlight(client *redis.Client, lockTTL time.Duration, resultTTL time.Duration) *RedisFlight {
	return &RedisFlight{client: client, lockTTL: lockTTL, resultTTL: resultTTL}
}

func (f *RedisFlight) Execute(ctx context.Context, key string, fn func(context.Context) (string, error)) (Result, error) {
	if f == nil || f.client == nil {
		value, err := fn(ctx)
		return Result{Value: value}, err
	}
	resultKey := "sf:result:" + key
	lockKey := "sf:lock:" + key

	if value, err := f.client.Get(ctx, resultKey).Result(); err == nil {
		return Result{Value: value, Replay: true}, nil
	}

	owned, err := f.client.SetNX(ctx, lockKey, "1", f.lockTTL).Result()
	if err != nil {
		value, runErr := fn(ctx)
		return Result{Value: value}, runErr
	}
	if !owned {
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(250 * time.Millisecond)
			if value, err := f.client.Get(ctx, resultKey).Result(); err == nil {
				return Result{Value: value, Replay: true}, nil
			}
		}
		return Result{}, ErrInFlight
	}
	defer f.client.Del(context.Background(), lockKey)

	value, err := fn(ctx)
	if err != nil {
		return Result{}, err
	}
	_ = f.client.Set(ctx, resultKey, value, f.resultTTL).Err()
	return Result{Value: value}, nil
}
