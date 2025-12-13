package cache

import (
	"context"
	"github.com/go-redis/redis/v8"
	"time"
)

type ICacheRepository interface {
	TxPipeline() redis.Pipeliner
	Get(context.Context, string, interface{}) (bool, error)
	Set(context.Context, string, interface{}, time.Duration) error
	Delete(context.Context, ...string) error
	Incr(context.Context, string) (int64, error)
	SetNX(context.Context, string, interface{}, time.Duration) (bool, error)
	Publish(ctx context.Context, channel string, message interface{}) error
	Subscribe(ctx context.Context, channel string) *redis.PubSub
}
