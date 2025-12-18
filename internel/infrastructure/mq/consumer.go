package mq

import (
	"GoChat/config"
	"GoChat/pkg/logger"
	"context"
	"errors"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"sync"
)

type Consumer interface {
	RegisterHandler(handler MessageHandler)
	Consume(ctx context.Context)
	Close()
}

// MessageHandler 定义业务处理函数的签名
type MessageHandler func(ctx context.Context, key, value []byte) error

type kafkaConsumer struct {
	r         *kafka.Reader
	handler   MessageHandler
	closeOnce sync.Once // 确保Close方法只执行一次
}

// NewKafkaConsumer 创建Kafka消费者实例
// 入参：全局配置、消费主题、业务处理函数
// 返参：消费者实例、错误
func NewKafkaConsumer(cfg *config.KafkaConfig, topic string) (Consumer, error) {
	// 1. 参数校验
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka brokers is empty")
	}
	if topic == "" {
		return nil, errors.New("kafka topic is empty")
	}
	//if cfg.GroupID == "" {
	//	return nil, errors.New("kafka group id is empty")
	//}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: cfg.Brokers,
		Topic:   topic,
		Logger:  logger.NewZapKafkaAdapter(logger.KafkaLogger.Sugar(), zap.InfoLevel),
	})

	return &kafkaConsumer{
		r: r,
	}, nil
}

func (kc *kafkaConsumer) RegisterHandler(handler MessageHandler) {
	kc.handler = handler
}

// Consume 启动消费者 (协程启动）
func (kc *kafkaConsumer) Consume(ctx context.Context) {
	logger.KafkaLogger.Info("开始执行消费者协程")
	for {
		select {
		case <-ctx.Done():
			kc.Close()
		default:
		}

		// 拉取消息
		msg, err := kc.r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.KafkaLogger.Warn("[Kafka] Fetch error", zap.Error(err))
			continue
		}

		// 执行业务处理
		if kc.handler != nil {
			if err = kc.handler(ctx, msg.Key, msg.Value); err != nil {
				logger.KafkaLogger.Error("[Kafka] 处理消息失败", zap.Error(err))
			}
		}

		if err = kc.r.CommitMessages(ctx, msg); err != nil {
			logger.KafkaLogger.Warn("[Kafka] Commit error", zap.Error(err))
		}
	}
}

func (kc *kafkaConsumer) Close() {
	kc.closeOnce.Do(func() {
		// 关闭kafka reader
		if kc.r != nil {
			if closeErr := kc.r.Close(); closeErr != nil {
				logger.KafkaLogger.Warn("[Kafka] reader close error", zap.Error(closeErr))
			}
		}
	})
}
