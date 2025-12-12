package db

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"time"
)

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

var redisClient *redis.Client

func GetRDS() *redis.Client {
	return redisClient
}

func StartRedis() {
	redisConfig := &RedisConfig{}
	if err := viper.UnmarshalKey("redis", redisConfig); err != nil {
		zap.L().Fatal("unmarshal redis config failed", zap.String("err", err.Error()))
	}
	var err error
	if redisClient, err = initRedis(redisConfig); err != nil {
		zap.L().Fatal("init redis failed", zap.String("err", err.Error()))
	}
	zap.L().Info("redis 初始化成功.")
}

func initRedis(redisCfg *RedisConfig) (*redis.Client, error) {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisCfg.Host, redisCfg.Port),
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
		PoolSize: redisCfg.PoolSize,
	})

	// 使用 Ping 命令测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return redisClient, nil
}

func CloseRedis() {
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			zap.L().Warn("redis close warn:", zap.Error(err))
		}
	}
}
