package router

import (
	"GoChat/internel/handler"
	"GoChat/internel/middleware"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	"GoChat/internel/service"
	"GoChat/pkg/db"
	"GoChat/pkg/util"
	"context"
	"github.com/gin-gonic/gin"
)

func RegisterWsRouter(r *gin.Engine) {
	dbs := db.GetDBS()
	rdb := db.GetRDS()

	userRepo := repository.NewUserRepo(dbs.Master)
	chatRepo := repository.NewChatRepo(dbs.Master)
	chatCache := cache.NewRedisCache(rdb)
	seqFactory := service.NewSeqFactory(chatCache)
	txManager := service.NewTxManager(dbs.Master)

	userService := service.NewUserService(userRepo, chatCache, txManager)
	pushService := service.NewPushService(chatCache, userService)
	chatService := service.NewChatService(seqFactory, pushService, chatRepo)

	wsHandler := handler.NewWsHandler(chatService, userService)

	{
		r.GET("/getAllClient", wsHandler.GetAllClient)
	}

	{
		chatApi := r.Group("/chat")
		chatApi.Use(middleware.JWTMiddleware())
		chatApi.GET("/ws", wsHandler.Connect)
	}

	// 启动redis订阅
	util.SafeGo(func() {
		pushService.Subscribe(context.Background(), util.PubSubChannel)
	})
}
