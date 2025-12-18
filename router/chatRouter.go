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

	// 进行kafka生产者依赖注入
	ackProducer, err := mq.NewKafkaProducer(&config.Cfg.KafkaConfig, mq.KafkaAckTopic)
	if err != nil {
		zap.L().Fatal("ack producer start error", zap.Error(err))
	}

	// 进行kafka消费者依赖注入
	ackConsumer, err := mq.NewKafkaConsumer(&config.Cfg.KafkaConfig, mq.KafkaAckTopic)
	if err != nil {
		zap.L().Fatal("ack consumer start error", zap.Error(err))
	}

	userService := service.NewUserService(userRepo, chatCache, txManager)
	pushService := service.NewPushService(chatCache, userService)
	chatService := service.NewChatService(seqFactory, pushService, chatRepo, ackProducer, ackConsumer)
	syncService := service.NewSyncService(chatCache, chatRepo, conversationRepo)

	chatHandler := handler.NewChatHandler(chatService, userService, syncService)

	// 注册并启动ack消费者

	//defer ackConsumer.Close()

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
