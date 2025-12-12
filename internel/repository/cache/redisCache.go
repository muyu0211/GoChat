package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"time"
)

var (
	ErrGet       = errors.New("redis get error")
	ErrUnmarshal = errors.New("redis unmarshal error")
	ErrMarshal   = errors.New("redis marshal error")
	ErrDel       = errors.New("redis del error")
	ErrPublish   = errors.New("redis publish error")
)

type redisCache struct {
	rdb *redis.Client
}

func NewRedisCache(rdb *redis.Client) ICacheRepository {
	return &redisCache{rdb: rdb}
}

// Set 通用方法
func (rc *redisCache) Set(ctx context.Context, key string, dest interface{}, expiration time.Duration) error {
	data, err := json.Marshal(dest)
	if err != nil {
		zap.L().Error("redis marshal error", zap.Error(err), zap.String("key", key))
		return ErrMarshal
	}
	return rc.rdb.Set(ctx, key, data, expiration).Err()
}

// Get 通用方法（自动反序列化）
func (rc *redisCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	data, err := rc.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil // 键不存在不视为错误
	} else if err != nil {
		zap.L().Error("redis get error", zap.Error(err), zap.String("key", key))
		return false, ErrGet
	}

	if err = json.Unmarshal(data, dest); err != nil {
		return true, ErrUnmarshal
	}
	return true, nil
}

func (rc *redisCache) Delete(ctx context.Context, keys ...string) error {
	err := rc.rdb.Del(ctx, keys...).Err()
	if err != nil {
		zap.L().Error("redis del error", zap.Error(err), zap.Strings("keys", keys))
		return ErrDel
	}
	return nil
}

func (rc *redisCache) Incr(ctx context.Context, key string) (int64, error) {
	id, err := rc.rdb.Incr(ctx, key).Result()
	if err != nil {
		zap.L().Warn(fmt.Sprintf("[INCR]-失败 key: %s ", key), zap.Error(err))
		return 0, err
	}
	return id, nil
}

func (rc *redisCache) SetNX(ctx context.Context, key string, dest interface{}, expireTime time.Duration) (bool, error) {
	// 进行序列化
	data, err := json.Marshal(dest)
	if err != nil {
		zap.L().Error("redis marshal error", zap.Error(err), zap.String("key", key))
		return false, ErrMarshal
	}
	success, err := rc.rdb.SetNX(ctx, key, data, expireTime).Result()
	if err != nil {
		zap.L().Warn(fmt.Sprintf("[SETNX]-失败 key: %s 设置", key), zap.Error(err))
	}
	return success, err
}

func (rc *redisCache) Publish(ctx context.Context, channel string, message interface{}) error {
	err := rc.rdb.Publish(ctx, channel, message).Err()
	if err != nil {
		zap.L().Warn(fmt.Sprintf("[PUBLISH]-失败 channel: %s 订阅", channel), zap.Error(err))
		return ErrPublish
	}
	return nil
}

func (rc *redisCache) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return rc.rdb.Subscribe(ctx, channel)
}
