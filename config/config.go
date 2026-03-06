package config

import (
	"bytes"
	"embed"
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	BasicConfig      `mapstructure:"app"`
	RedisConfig      `mapstructure:"redis"`
	MySQLConfig      `mapstructure:"mysql"`
	KafkaConfig      `mapstructure:"kafka"`
	LoggerConfig     `mapstructure:"logger"`
	GormLoggerConfig `mapstructure:"gorm_logger"`
	JWTConfig        `mapstructure:"jwt"`
	AntsConfig       `mapstructure:"ants"`
}

type BasicConfig struct {
	ServerID uint64 `mapstructure:"server_id"`
	Name     string `mapstructure:"name"`
	Port     string `mapstructure:"port"`
	Mode     string `mapstructure:"mode"`
}

type MySQLConfig struct {
	Master          DBConfig `mapstructure:"master"`
	Slave           DBConfig `mapstructure:"slave"`
	MaxOpenConns    int      `mapstructure:"max_open_conns"`
	MaxIdleConns    int      `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int      `mapstructure:"conn_max_lifetime"`
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

type GormLoggerConfig struct {
	Filename      string        `mapstructure:"filename"`       // 日志文件名称
	SlowThreshold time.Duration `mapstructure:"slow_threshold"` // 慢查询阈值
	MaxSize       int           `mapstructure:"max_size"`       // 日志文件最大大小
	MaxAge        int           `mapstructure:"max_age"`        // 日志文件最大年龄
	MaxBackups    int           `mapstructure:"max_backups"`    // 最多保留的备份文件数
	Compress      bool          `mapstructure:"compress"`       // 是否压缩备份文件
}

type KafkaLoggerConfig struct {
	Filename   string `mapstructure:"filename"`    // 日志文件名称
	MaxSize    int    `mapstructure:"max_size"`    // 日志文件最大大小
	MaxAge     int    `mapstructure:"max_age"`     // 日志文件最大年龄
	MaxBackups int    `mapstructure:"max_backups"` // 最多保留的备份文件数
	Compress   bool   `mapstructure:"compress"`    // 是否压缩备份文件
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

//go:embed config.yml
var configFS embed.FS

// LoadConfig 加载所有配置
func LoadConfig() *Config {
	configBytes, err := configFS.ReadFile("config.yml")
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
	// 3. 告诉 viper 配置文件的类型（因为此时是从内存读取，viper 无法通过文件名后缀推断格式）
	viper.SetConfigType("yml") // 或者 "yaml"
	// 4. 从字节流中读取配置
	if err := viper.ReadConfig(bytes.NewReader(configBytes)); err != nil {
		panic(fmt.Errorf("fatal error parsing config file: %w", err))
	}
	if err := viper.Unmarshal(&Cfg); err != nil {
		panic(fmt.Errorf("unable to decode into struct: %w", err))
	}

	// 初始化其他配置
	LoadKafkaConfig(Cfg)
	return Cfg
}
