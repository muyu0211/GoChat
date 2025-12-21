package mq

import (
	"time"
)

// ProducerOptions 生产者通用配置参数
// 这是 mq 包定义的标准，与具体业务解耦
type ProducerOptions struct {
	Brokers      []string
	Topic        string
	BatchSize    int
	BatchTimeout time.Duration
	WriteTimeout time.Duration
	Compression  string // "snappy", "gzip", "none"
}

// ConsumerOptions 消费者通用配置参数
type ConsumerOptions struct {
	Brokers        []string
	Topic          string
	GroupID        string
	MinBytes       int
	MaxBytes       int
	CommitInterval time.Duration
	StartOffset    int64 // -1 (First), -2 (Last)
}

// DefaultProducerOptions 默认配置生成器 (可选)
func DefaultProducerOptions() ProducerOptions {
	return ProducerOptions{
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		WriteTimeout: 10 * time.Second,
		Compression:  "snappy",
	}
}
