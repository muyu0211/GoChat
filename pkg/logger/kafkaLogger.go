package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
)

type zapKafkaAdapter struct {
	logger *zap.SugaredLogger
	level  zapcore.Level
}

var (
	KafkaLogger *zap.Logger // Kafka专属日志实例
)

// 初始化Kafka日志
func init() {
	env := "dev"

	// 1. 配置日志切割 (Lumberjack)
	kafkaWriteSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "logs/kafka.log", // 单独的日志文件路径
		MaxSize:    100,              // 每个日志文件最大 100MB
		MaxBackups: 10,               // 保留最近 10 个文件
		MaxAge:     30,               // 保留最近 30 天
		Compress:   true,             // 是否压缩
	})

	// 2. 配置编码器 (Encoder) - 建议开发环境用 Console，生产用 JSON
	var writeSyncer zapcore.WriteSyncer
	var encoder zapcore.Encoder
	if env == "dev" {
		// 开发环境：控制台友好的格式（彩色、换行）
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // 彩色级别
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
		writeSyncer = zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)) // 同时写入文件中：kafkaWriteSyncer
	} else {
		// 生产环境：JSON格式（便于日志收集工具解析）
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "timestamp"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // 标准化时间格式
		encoder = zapcore.NewJSONEncoder(encoderConfig)
		writeSyncer = kafkaWriteSyncer
	}

	// 5. 创建Kafka日志的Core（级别：Debug及以上，便于排查Kafka问题）
	kafkaCore := zapcore.NewCore(
		encoder,
		writeSyncer,
		zapcore.DebugLevel,
	)

	KafkaLogger = zap.New(kafkaCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)).
		With(zap.String("module", "kafka"))
}

func NewZapKafkaAdapter(logger *zap.SugaredLogger, level zapcore.Level) *zapKafkaAdapter {
	return &zapKafkaAdapter{
		logger: logger,
		level:  level,
	}
}

// Printf 实现kafka.Logger的Printf方法（转发到KafkaLogger）
func (z *zapKafkaAdapter) Printf(format string, v ...interface{}) {
	switch z.level {
	case zapcore.DebugLevel:
		z.logger.Debugf(format, v...)
	case zapcore.WarnLevel:
		z.logger.Warnf(format, v...)
	case zapcore.ErrorLevel:
		z.logger.Errorf(format, v...)
	default:
		z.logger.Infof(format, v...)
	}
}
