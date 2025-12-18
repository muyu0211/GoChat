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

type ICacheRepository interface {
	TxPipeline() redis.Pipeliner
	Get(context.Context, string, interface{}) (bool, error)
	Set(context.Context, string, interface{}, time.Duration) error
	Delete(context.Context, ...string) error
	Incr(context.Context, string) (int64, error)
	SetNX(context.Context, string, interface{}, time.Duration) (bool, error)
	Publish(ctx context.Context, channel string, message interface{}) error
	Subscribe(ctx context.Context, channel string) *redis.PubSub
	ZAdd(ctx context.Context, key string, score float64, member []byte, expire time.Duration) error
	ZRange(ctx context.Context, key string, start uint64, limit int64) ([][]byte, error)
	HSet(ctx context.Context, key string, fields ...map[string]interface{}) error
	HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error)
	SAdd(ctx context.Context, key string, members ...interface{}) error
	SMembers(ctx context.Context, key string) ([]string, error)
	SRem(ctx context.Context, key string, members ...interface{}) error
	SMembersWithCheck(ctx context.Context, key string) ([]string, bool, error)
}

var (
	ErrGet       = errors.New("cache get error")
	ErrUnmarshal = errors.New("cache unmarshal error")
	ErrMarshal   = errors.New("cache marshal error")
	ErrDel       = errors.New("cache del error")
	ErrPublish   = errors.New("cache publish error")
)

type redisCache struct {
	rdb *redis.Client
}

func NewRedisCache(rdb *redis.Client) ICacheRepository {
	return &redisCache{rdb: rdb}
}

func (rc *redisCache) TxPipeline() redis.Pipeliner {
	return rc.rdb.TxPipeline()
}

// Set 通用方法
func (rc *redisCache) Set(ctx context.Context, key string, dest interface{}, expiration time.Duration) error {
	data, err := json.Marshal(dest)
	if err != nil {
		zap.L().Error("cache marshal error", zap.Error(err), zap.String("key", key))
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
		zap.L().Error("cache get error", zap.Error(err), zap.String("key", key))
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
		zap.L().Error("cache del error", zap.Error(err), zap.Strings("keys", keys))
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
		zap.L().Error("cache marshal error", zap.Error(err), zap.String("key", key))
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

func (rc *redisCache) ZAdd(ctx context.Context, key string, score float64, member []byte, expire time.Duration) error {
	pipe := rc.rdb.Pipeline()
	pipe.ZAdd(ctx, key, &redis.Z{
		Score:  score,
		Member: member,
	})
	if expire != 0 {
		pipe.Expire(ctx, key, expire)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (rc *redisCache) ZRange(ctx context.Context, key string, start uint64, limit int64) ([][]byte, error) {
	// Range: (minSeq, +inf]
	// 注意：Redis ZRangeByScore 默认是闭区间，我们需要开区间，所以用 "(" + minSeq
	opt := &redis.ZRangeBy{
		Min:   fmt.Sprintf("(%d", start),
		Max:   "+inf",
		Count: int64(limit), // Limit 限制
	}
	// 查询redis
	memStr, err := rc.rdb.ZRangeByScore(ctx, key, opt).Result()
	if err != nil {
		return nil, err
	}
	res := make([][]byte, 0)
	for _, str := range memStr {
		res = append(res, []byte(str))
	}
	return res, nil
}

func (rc *redisCache) HSet(ctx context.Context, key string, fields ...map[string]interface{}) error {
	_, err := rc.rdb.HSet(ctx, key, fields).Result()
	if err != nil {
		return err
	}
	return nil
}

func (rc *redisCache) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	result, err := rc.rdb.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, err
	}
	return result, err
}

func (rc *redisCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	_, err := rc.rdb.SAdd(ctx, key, members...).Result()
	if err != nil {
		return err
	}
	return nil
}

func (rc *redisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	result, err := rc.rdb.SMembers(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	return result, err
}

func (rc *redisCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	_, err := rc.rdb.SRem(ctx, key, members...).Result()
	if err != nil {
		return err
	}
	return nil
}

func (rc *redisCache) SMembersWithCheck(ctx context.Context, key string) ([]string, bool, error) {
	// 1. 先检查键是否存在
	exist, err := rc.rdb.Exists(ctx, key).Result()
	if err != nil {
		return nil, false, err
	}
	if exist == 0 {
		return nil, false, nil
	}

	// 2.获取集合成员
	cmd := rc.rdb.SMembers(ctx, key)
	if cmd.Err() != nil {
		return nil, false, cmd.Err()
	}
	return cmd.Val(), true, nil
}

func (rc *redisCache) ZRemRange(ctx context.Context, key string, score interface{}) error {
	err := rc.rdb.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%v", score)).Err()
	return err
}
