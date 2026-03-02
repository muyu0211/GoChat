package mq

import (
	"GoChat/config"
)

// 包装类区分不同的消费者对象

type AckConsumer struct {
	Consumer
}

type AckProducer struct {
	Producer
}

type LogConsumer struct {
	Consumer
}

type LogProducer struct {
	Producer
}

type GroupMsgConsumer struct {
	Consumer
}

type GroupMsgProducer struct {
	Producer
}

func NewAckConsumer(brokers []string, cfg *config.BusinessConfig) (*AckConsumer, error) {
	consumer, err := NewKafkaConsumer(brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &AckConsumer{Consumer: consumer}, nil
}

func NewAckProducer(brokers []string, cfg *config.BusinessConfig) (*AckProducer, error) {
	producer, err := NewKafkaProducer(brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &AckProducer{Producer: producer}, nil
}

func NewGroupMsgConsumer(brokers []string, cfg *config.BusinessConfig) (*GroupMsgConsumer, error) {
	consumer, err := NewKafkaConsumer(brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &GroupMsgConsumer{Consumer: consumer}, nil
}

func NewGroupMsgProducer(brokers []string, cfg *config.BusinessConfig) (*GroupMsgProducer, error) {
	producer, err := NewKafkaProducer(brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &GroupMsgProducer{Producer: producer}, nil
}
