package cache

import (
	"GoChat/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// 预编译 Lua 脚本（提升性能，避免每次执行都解析脚本）
var dedupAndSeqScript = redis.NewScript(DedupAndSeqLua)

type ICacheRepository interface {
	TxPipeline() redis.Pipeliner
	Exists(ctx context.Context, keys ...string) (bool, error)
	Get(context.Context, string, interface{}) (bool, error)
	Set(context.Context, string, interface{}, time.Duration) error
	Delete(context.Context, ...string) error
	Incr(context.Context, string) (int64, error)
	IncrBy(context.Context, string, int64) (int64, error)
	SetNX(context.Context, string, interface{}, time.Duration) (bool, error)
	Publish(ctx context.Context, channel string, message interface{}) error
	Subscribe(ctx context.Context, channel string) *redis.PubSub
	ZAdd(ctx context.Context, key string, score float64, member []byte, expire time.Duration) error
	ZRange(ctx context.Context, key string, start uint64, limit int64) ([][]byte, bool, error)
	HSet(ctx context.Context, key string, TTL time.Duration, fields map[string]interface{}) error
	HGet(ctx context.Context, key string, field string) (string, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error)
	SAdd(ctx context.Context, key string, expire time.Duration, members interface{}) error
	SMembers(ctx context.Context, key string) ([]string, error)
	SRem(ctx context.Context, key string, members interface{}) error
	SMembersWithCheck(ctx context.Context, key string) ([]string, bool, error)
	ZRemRange(ctx context.Context, key string, score interface{}) error

	DedupAndSeq(ctx context.Context, keys []string, expire time.Duration) (int64, error)
	UpdateUserConv(ctx context.Context, key string, expire time.Duration, convIDs []string) error
}

var (
	ErrGet       = errors.New("cache get error")
	ErrUnmarshal = errors.New("cache unmarshal error")
	ErrMarshal   = errors.New("cache marshal error")
	ErrDel       = errors.New("cache del error")
	ErrPublish   = errors.New("cache publish error")
	ErrType      = errors.New("data type error")
)

type RedisCache struct {
	rdb *redis.Client
}

func NewRedisCache(rdb *redis.Client) *RedisCache {
	return &RedisCache{rdb: rdb}
}

func (rc *RedisCache) TxPipeline() redis.Pipeliner {
	return rc.rdb.TxPipeline()
}

func (rc *RedisCache) Exists(ctx context.Context, keys ...string) (bool, error) {
	result, err := rc.rdb.Exists(ctx, keys...).Result()
	if err != nil {
		zap.L().Error("cache get error", zap.Error(err), zap.Strings("keys", keys))
		return false, ErrGet
	}
	return result > 0, nil
}

// Set 通用方法
func (rc *RedisCache) Set(ctx context.Context, key string, dest interface{}, expiration time.Duration) error {
	data, err := json.Marshal(dest)
	if err != nil {
		zap.L().Error("cache marshal error", zap.Error(err), zap.String("key", key))
		return ErrMarshal
	}
	return rc.rdb.Set(ctx, key, data, expiration).Err()
}

// Get 通用方法（自动反序列化）
func (rc *RedisCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
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

func (rc *RedisCache) Delete(ctx context.Context, keys ...string) error {
	err := rc.rdb.Del(ctx, keys...).Err()
	if err != nil {
		zap.L().Error("cache del error", zap.Error(err), zap.Strings("keys", keys))
		return ErrDel
	}
	return nil
}

func (rc *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	id, err := rc.rdb.Incr(ctx, key).Result()
	if err != nil {
		zap.L().Warn(fmt.Sprintf("[INCR]-失败 key: %s ", key), zap.Error(err))
		return 0, err
	}
	return id, nil
}

func (rc *RedisCache) IncrBy(ctx context.Context, key string, step int64) (int64, error) {
	id, err := rc.rdb.IncrBy(ctx, key, step).Result()
	if err != nil {
		zap.L().Warn(fmt.Sprintf("[INCRBY]-失败 key: %s 步长: %d", key, step), zap.Error(err))
		return 0, err
	}
	return id, nil
}

func (rc *RedisCache) SetNX(ctx context.Context, key string, dest interface{}, expireTime time.Duration) (bool, error) {
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

func (rc *RedisCache) Publish(ctx context.Context, channel string, message interface{}) error {
	err := rc.rdb.Publish(ctx, channel, message).Err()
	if err != nil {
		zap.L().Warn(fmt.Sprintf("[PUBLISH]-失败 channel: %s 订阅", channel), zap.Error(err))
		return ErrPublish
	}
	return nil
}

func (rc *RedisCache) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return rc.rdb.Subscribe(ctx, channel)
}

func (rc *RedisCache) ZAdd(ctx context.Context, key string, score float64, member []byte, expire time.Duration) error {
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

func (rc *RedisCache) ZRange(ctx context.Context, key string, start uint64, limit int64) ([][]byte, bool, error) {
	exist, err := rc.rdb.Exists(ctx, key).Result()
	if err != nil {
		return nil, false, err
	}
	if exist == 0 {
		return nil, false, nil
	}

	// Range: [minSeq, +inf]
	opt := &redis.ZRangeBy{
		Min:   fmt.Sprintf("%d", start),
		Max:   "+inf",
		Count: limit,
	}
	// 查询redis
	memStr, err := rc.rdb.ZRangeByScore(ctx, key, opt).Result()
	if err != nil {
		zap.L().Error("cache get error", zap.Error(err), zap.String("key", key))
		return nil, false, err
	}
	res := make([][]byte, 0)
	for _, str := range memStr {
		res = append(res, []byte(str))
	}
	return res, true, nil
}

func (rc *RedisCache) HSet(ctx context.Context, key string, TTL time.Duration, fields map[string]interface{}) error {
	pipe := rc.rdb.Pipeline()
	pipe.HSet(ctx, key, fields)
	if TTL != 0 {
		pipe.Expire(ctx, key, TTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (rc *RedisCache) HGet(ctx context.Context, key string, field string) (string, error) {
	result, err := rc.rdb.HGet(ctx, key, field).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	return result, err
}
func (rc *RedisCache) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	result, err := rc.rdb.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, err
	}
	return result, err
}

func (rc *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	result, err := rc.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		zap.L().Error("cache get error", zap.Error(err), zap.String("key", key))
	}
	return result, err
}

func (rc *RedisCache) SAdd(ctx context.Context, key string, expire time.Duration, members interface{}) error {
	args := util.SliceToIfaceSlice(members)
	pipe := rc.rdb.Pipeline()
	pipe.SAdd(ctx, key, args)
	if expire != 0 {
		pipe.Expire(ctx, key, expire)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (rc *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	result, err := rc.rdb.SMembers(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	return result, err
}

// SRem 删除集合元素
func (rc *RedisCache) SRem(ctx context.Context, key string, members interface{}) error {
	args := util.SliceToIfaceSlice(members)
	_, err := rc.rdb.SRem(ctx, key, args).Result()
	if err != nil {
		return err
	}
	return nil
}

func (rc *RedisCache) SMembersWithCheck(ctx context.Context, key string) ([]string, bool, error) {
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

// ZRemRange 删除小于指定分数的元素
func (rc *RedisCache) ZRemRange(ctx context.Context, key string, score interface{}) error {
	err := rc.rdb.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%v", score)).Err()
	return err
}

func (rc *RedisCache) DedupAndSeq(ctx context.Context, keys []string, expire time.Duration) (int64, error) {
	var v int64
	var ok bool
	if ret, err := dedupAndSeqScript.Run(ctx, rc.rdb, keys, int64(expire.Seconds())).Result(); err != nil {
		return 0, err
	} else if v, ok = ret.(int64); !ok {
		return 0, ErrType
	}
	return v, nil

}

// UpdateUserConv 更新用户会话列表
func (rc *RedisCache) UpdateUserConv(ctx context.Context, key string, expire time.Duration, convIDs []string) error {
	// 使用lua脚本：先对Set进行删除，再对Set进行添加
	return nil
}
