package mq

import (
	"GoChat/config"
	"GoChat/pkg/logger"
	"GoChat/pkg/util"
	"context"
	"errors"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"log"
)

type Producer interface {
	Publish(ctx context.Context, topic string, key, msg []byte) error
}

type kafkaProducer struct {
	w *kafka.Writer
}

func NewKafkaProducer(cfg *config.KafkaConfig, topic string) (Producer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka brokers is empty")
	}
	if topic == "" {
		return nil, errors.New("kafka topic is empty")
	}

	w := &kafka.Writer{
		Addr:                   nil,
		Topic:                  topic,
		Balancer:               nil,
		WriteTimeout:           0,
		RequiredAcks:           kafka.RequireNone,
		AllowAutoTopicCreation: false,
		Logger:                 logger.NewZapKafkaAdapter(logger.KafkaLogger.Sugar(), zap.InfoLevel),
	}

	return &kafkaProducer{
		w: w,
	}, nil
}

// Publish 发布消息
func (kp *kafkaProducer) Publish(ctx context.Context, topic string, key, msg []byte) error {
	// 设置要写入的topic
	kp.w.Topic = topic
	// 带重试的写入
	if err := util.Retry(util.RetryMaxTimes, util.RetryInterval, func() error {
		log.Println("向kafka发布消息：", key, string(msg))
		return kp.w.WriteMessages(ctx, kafka.Message{Key: key, Value: msg})
	}); err != nil {
		zap.L().Warn("kafka publish error", zap.Error(err))
	}
	return nil
}
