package logger

import (
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"reflect"
)

type LoggerConfig struct {
	Level      string `mapstructure:"level"`    // 配置日志阈值级别
	Filename   string `mapstructure:"filename"` // 日志文件名称
	MaxSize    int    `mapstructure:"max_size"` // 日志文件最大大小
	MaxAge     int    `mapstructure:"max_age"`
	MaxBackups int    `mapstructure:"max_backups"`
	Compress   bool   `mapstructure:"compress"`
}

// StartLogger 启动日志服务
func StartLogger() {
	logCfg := &LoggerConfig{}
	if err := viper.UnmarshalKey("logger", logCfg); err != nil {
		panic(fmt.Errorf("unable to decode into %s, %v", reflect.TypeOf(logCfg).Name(), err))
	}
	// 初始化日志服务
	initLogger(logCfg)
}

// initLogger 初始化日志服务
func initLogger(logCfg *LoggerConfig) {
	core := zapcore.NewCore(getEncoder(), getWriteSyncer(logCfg), getLevelEnabler(logCfg))
	logger := zap.New(core, zap.AddCaller())

	// 替换zap的全局logger对象
	zap.ReplaceGlobals(logger)
	zap.L().Info("日志初始化成功.")
}

// CloseLogger 关闭日志服务
func CloseLogger() {
	if err := zap.L().Sync(); err != nil { // 确保日志刷盘
		zap.L().Warn("LOGGER SYNC WARN:", zap.Error(err))
	}
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder, // 小写编码器
		EncodeTime:     zapcore.ISO8601TimeEncoder,    // ISO8601 UTC 时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder, // 短路径编码器
	}

	// 开发环境下则进行控制台打印
	if viper.GetString("mode") == "dev" {
		return zapcore.NewConsoleEncoder(encoderConfig)
	}

	return zapcore.NewJSONEncoder(encoderConfig)
}

func getWriteSyncer(cfg *LoggerConfig) zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   cfg.Filename,   // 日志文件路径
		MaxSize:    cfg.MaxSize,    // 每个日志文件的最大尺寸，单位：MB
		MaxBackups: cfg.MaxBackups, // 保留的旧日志文件的最大数量
		MaxAge:     cfg.MaxAge,     // 旧日志文件的最长保留天数
		Compress:   cfg.Compress,   // 是否压缩旧日志文件
	}
	// 在开发环境下，同时输出到控制台和文件
	if viper.GetString("mode") == "dev" {
		return zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(lumberJackLogger))
	}

	return zapcore.AddSync(lumberJackLogger)
}

func getLevelEnabler(cfg *LoggerConfig) zapcore.Level {
	switch cfg.Level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}
