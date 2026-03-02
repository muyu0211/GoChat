package logger

import (
	"GoChat/config"
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type zapGormAdapter struct {
	logger        *zap.Logger
	SlowThreshold time.Duration
}

// NewGormLogger 创建一个专门用于Gorm的Zap Logger，输出到指定文件
func NewGormLogger(cfg *config.GormLoggerConfig) logger.Interface {
	// 1. 配置日志切割 (Lumberjack)
	writeSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.Filename,   // 单独的日志文件路径
		MaxSize:    cfg.MaxSize,    // 每个日志文件最大 100MB
		MaxBackups: cfg.MaxBackups, // 保留最近 10 个文件
		MaxAge:     cfg.MaxAge,     // 保留最近 30 天
		Compress:   cfg.Compress,   // 是否压缩
	})

	// 2. 配置编码器 (Encoder) - 建议开发环境用 Console，生产用 JSON
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder   // 时间格式 2023-01-01T...
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // INFO, WARN

	// 如果你希望 SQL 日志是纯文本方便看，可以用 ConsoleEncoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// 3. 创建 Core
	// 注意：这里 Level 设置为 DebugLevel，因为你的适配器中正常 SQL 是用 Debug 输出的
	core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)

	// 4. 创建 Logger 核心
	l := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(3))

	return &zapGormAdapter{
		logger:        l,
		SlowThreshold: cfg.SlowThreshold,
	}
}

func (l *zapGormAdapter) LogMode(level logger.LogLevel) logger.Interface {
	return l
}

func (l *zapGormAdapter) Info(ctx context.Context, msg string, data ...interface{}) {
	l.logger.Info(msg, zap.Any("data", data))
}

func (l *zapGormAdapter) Warn(ctx context.Context, msg string, data ...interface{}) {
	l.logger.Warn(msg, zap.Any("data", data))
}

func (l *zapGormAdapter) Error(ctx context.Context, msg string, data ...interface{}) {
	l.logger.Error(msg, zap.Any("data", data))
}

func (l *zapGormAdapter) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := []zapcore.Field{
		zap.String("sql", sql),
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
	}

	// 慢查询日志
	if elapsed > l.SlowThreshold {
		l.logger.Warn("SLOW SQL", fields...)
		return
	}

	// 错误日志
	if err != nil {
		// 如果是“记录未找到”，降级为 Warn 或 Info，或者直接忽略（取决于你的业务需求）
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			fields = append(fields, zap.Error(err))
			l.logger.Error("SQL ERROR", fields...)
		}
		return
	}

	// 正常日志
	l.logger.Debug("SQL", fields...)
}
