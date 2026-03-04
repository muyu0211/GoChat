package mq

import (
	"GoChat/config"
	"GoChat/pkg/util"
	"context"
	"errors"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Producer interface {
	Publish(ctx context.Context, key, msg []byte) error
}

type kafkaProducer struct {
	w *kafka.Writer
}

func NewKafkaProducer(brokers []string, cfg *config.BusinessConfig) (Producer, error) {
	if len(brokers) == 0 {
		return nil, errors.New("invalid producer options")
	}

	// 映射压缩算法
	var compressor kafka.Compression
	switch cfg.Producer.Compression {
	case "snappy":
		compressor = kafka.Snappy
	case "gzip":
		compressor = kafka.Gzip
	}

	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.Hash{}, // 默认使用 Hash 负载均衡
		BatchSize:    cfg.Producer.BatchSize,
		BatchTimeout: cfg.Producer.BatchTimeout,
		WriteTimeout: cfg.Producer.WriteTimeout,
		Compression:  compressor,
		Async:        true,
		//Logger:         logger.NewZapKafkaAdapter(logger.KafkaLogger.Sugar(), zap.DebugLevel),
	}

	return &kafkaProducer{w: w}, nil
}

// Publish 发布消息
func (kp *kafkaProducer) Publish(ctx context.Context, key, msg []byte) error {
	if err := util.Retry(util.RetryMaxTimes, util.RetryInterval, func() error {
		return kp.w.WriteMessages(ctx, kafka.Message{Key: key, Value: msg})
	}); err != nil {
		zap.L().Warn("kafka publish error", zap.Error(err))
	}
	return nil
}
