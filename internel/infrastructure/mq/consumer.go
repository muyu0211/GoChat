package mq

import (
	"GoChat/config"
	"GoChat/pkg/logger"
	"context"
	"errors"
	"sync"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
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
	closeOnce sync.Once
}

// NewKafkaConsumer 创建Kafka消费者实例
func NewKafkaConsumer(brokers []string, cfg *config.BusinessConfig) (Consumer, error) {
	if len(brokers) == 0 || cfg.GroupID == "" || cfg.Topic == "" {
		return nil, errors.New("invalid consumer options")
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       cfg.Consumer.MinBytes,
		MaxBytes:       cfg.Consumer.MaxBytes,
		MaxWait:        cfg.Consumer.MaxWait,
		CommitInterval: cfg.Consumer.CommitInterval,
		StartOffset:    cfg.Consumer.StartOffset,
		// Logger:         logger.NewZapKafkaAdapter(logger.KafkaLogger.Sugar(), zap.DebugLevel),
	})
	return &kafkaConsumer{r: r}, nil
}

func (kc *kafkaConsumer) RegisterHandler(handler MessageHandler) {
	kc.handler = handler
}

// Consume 启动消费者 (协程启动)
func (kc *kafkaConsumer) Consume(ctx context.Context) {
	logger.KafkaLogger.Info("开始执行消费者协程")
	for {
		// 0. 监听服务是否关闭
		select {
		case <-ctx.Done():
			kc.Close()
		default:
		}

		// 1.拉取消息
		msg, err := kc.r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.KafkaLogger.Warn("[Kafka] Fetch error", zap.Error(err))
			continue
		}

		// 2.执行业务处理
		if kc.handler != nil {
			if err = kc.handler(ctx, msg.Key, msg.Value); err != nil {
				logger.KafkaLogger.Error("[Kafka] 处理消息失败", zap.Error(err))
			}
		}

		// 3.消息确认
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
