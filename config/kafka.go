package config

import "time"

type KafkaConfig struct {
	Brokers   []string  `mapstructure:"brokers"`
	AckConfig AckConfig `mapstructure:"ack"`
}

// AckConfig 针对 ACK 业务的特定配置
type AckConfig struct {
	Topic    string         `mapstructure:"topic"`
	GroupID  string         `mapstructure:"group_id"`
	Producer ProducerConfig `mapstructure:"producer"`
	Consumer ConsumerConfig `mapstructure:"consumer"`
}

// ProducerConfig 生产者调优参数
type ProducerConfig struct {
	BatchSize    int           `mapstructure:"batch_size"`    // 批量发送条数 (如 100)
	BatchTimeout time.Duration `mapstructure:"batch_timeout"` // 批量发送时间间隔 (如 10ms)
	WriteTimeout time.Duration `mapstructure:"write_timeout"` // 网络写超时
	Compression  string        `mapstructure:"compression"`   // 压缩算法: none, gzip, snappy, lz4, zstd
}

// ConsumerConfig 消费者调优参数
type ConsumerConfig struct {
	MinBytes       int           `mapstructure:"min_bytes"`       // 最小拉取字节 (如 10KB)
	MaxBytes       int           `mapstructure:"max_bytes"`       // 最大拉取字节 (如 10MB)
	CommitInterval time.Duration `mapstructure:"commit_interval"` // 自动提交间隔
	Workers        int           `mapstructure:"workers"`         // 【关键】消费者内部处理协程池大小
}
