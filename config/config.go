package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	BasicConfig  `mapstructure:"app"`
	RedisConfig  `mapstructure:"redis"`
	MySQLConfig  `mapstructure:"mysql"`
	KafkaConfig  `mapstructure:"kafka"`
	LoggerConfig `mapstructure:"logger"`
	JWTConfig    `mapstructure:"jwt"`
	AntsConfig   `mapstructure:"ants"`
}

type BasicConfig struct {
	ServerID uint64 `mapstructure:"server_id"`
	Name     string `mapstructure:"name"`
	Port     string `mapstructure:"port"`
	Mode     string `mapstructure:"mode"`
}

type MySQLConfig struct {
	Master          DBConfig      `mapstructure:"master"`
	Slave           DBConfig      `mapstructure:"slave"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int           `mapstructure:"conn_max_lifetime"`
	SlowThreshold   time.Duration `mapstructure:"slow_threshold"`
}

type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type LoggerConfig struct {
	Level      string `mapstructure:"level"`    // 配置日志阈值级别
	Filename   string `mapstructure:"filename"` // 日志文件名称
	MaxSize    int    `mapstructure:"max_size"` // 日志文件最大大小
	MaxAge     int    `mapstructure:"max_age"`
	MaxBackups int    `mapstructure:"max_backups"`
	Compress   bool   `mapstructure:"compress"`
}

type JWTConfig struct {
	JwtSecret   string `mapstructure:"secret"`       // jwt密钥
	ExpireHours int    `mapstructure:"expire_hours"` // token过期时间
	Issuer      string `mapstructure:"issuer"`       // 签发人
	SignMethod  string `mapstructure:"sign_method"`  // 签名方法
}

type AntsConfig struct {
	PoolSize       int           `mapstructure:"pool_size"`
	ExpiryDuration time.Duration `mapstructure:"expiry_duration"`
}

var Cfg *Config

// LoadConfig 加载所有配置
func LoadConfig() *Config {
	// 使用viper读取配置文件，进行项目初始化
	// ./config.config.yml
	viper.SetConfigFile("./config/config.yml")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
	if err := viper.Unmarshal(&Cfg); err != nil {
		panic(fmt.Errorf("unable to decode into struct: %w", err))
	}

	// 初始化其他配置
	LoadKafkaConfig(Cfg)
	return Cfg
}
