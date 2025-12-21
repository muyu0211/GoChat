package router

import (
	"GoChat/config"
	"GoChat/internel/handler"
	"GoChat/internel/infrastructure/mq"
	"GoChat/internel/middleware"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	"GoChat/internel/service"
	"GoChat/pkg/db"
	"GoChat/pkg/util"
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RegisterChatRouter(r *gin.Engine) {
	dbs := db.GetDBS()
	rdb := db.GetRDS()

	userRepo := repository.NewUserRepo(dbs.Master)
	chatRepo := repository.NewChatRepo(dbs.Master)
	conversationRepo := repository.NewConversationRepo(dbs.Master)
	chatCache := cache.NewRedisCache(rdb)
	seqFactory := service.NewSeqFactory(chatCache)
	txManager := service.NewTxManager(dbs.Master)

	// 进行kafka生产者/消费者依赖注入
	ackProducer, err := mq.NewKafkaProducer(config.Cfg.KafkaConfig.Brokers, &config.Cfg.KafkaConfig.AckConfig)
	if err != nil {
		zap.L().Fatal("ack producer start error", zap.Error(err))
	}
	ackConsumer, err := mq.NewKafkaConsumer(config.Cfg.KafkaConfig.Brokers, &config.Cfg.KafkaConfig.AckConfig)
	if err != nil {
		zap.L().Fatal("ack consumer start error", zap.Error(err))
	}
	userService := service.NewUserService(userRepo, chatCache, txManager)
	pushService := service.NewPushService(chatCache, userService)
	chatService := service.NewChatService(seqFactory, pushService, chatRepo, conversationRepo, txManager, chatCache, ackProducer, ackConsumer)
	syncService := service.NewSyncService(chatCache, chatRepo, conversationRepo)

	chatHandler := handler.NewChatHandler(chatService, userService, syncService)

	{
		r.GET("/getAllClient", chatHandler.GetAllClient)
	}

	{
		chatApi := r.Group("/chat")
		chatApi.Use(middleware.JWTMiddleware())
		chatApi.GET("/ws", chatHandler.Connect)
		chatApi.GET("/sessions", chatHandler.SyncSession)
	}

	// 启动redis订阅
	util.SafeGo(func() {
		pushService.Subscribe(context.Background(), util.PubSubChannel)
	})
	// 启动消费者监听
	util.SafeGo(func() {
		chatService.Run(context.Background())
	})
}

// buildProdConsOptions 根据业务生成生产者配置参数
func buildProdConsOptions(cfg config.KafkaConfig, businessCfg config.BusinessConfig) (mq.ProducerOptions, mq.ConsumerOptions) {
	brokers := cfg.Brokers
	producerOpts := mq.ProducerOptions{
		Brokers:      brokers,
		Topic:        businessCfg.Topic,
		BatchSize:    businessCfg.Producer.BatchSize,
		BatchTimeout: businessCfg.Producer.BatchTimeout,
		WriteTimeout: businessCfg.Producer.WriteTimeout,
		Compression:  businessCfg.Producer.Compression,
	}
	consumerOpts := mq.ConsumerOptions{
		Brokers:        brokers,
		Topic:          businessCfg.Topic,
		GroupID:        businessCfg.GroupID,
		MinBytes:       businessCfg.Consumer.MinBytes,
		MaxBytes:       businessCfg.Consumer.MaxBytes,
		CommitInterval: businessCfg.Consumer.CommitInterval,
		StartOffset:    businessCfg.Consumer.StartOffset,
	}
	return producerOpts, consumerOpts
}
