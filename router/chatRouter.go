package router

import (
	"GoChat/internel/handler"
	"GoChat/internel/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterChatRouter(r *gin.Engine, chatHandler *handler.ChatHandler) {
	// 1. 公共路由
	{
		r.GET("/get_all_client", chatHandler.GetAllClient)
	}

	// 2. Chat 组路由
	{
		chatApi := r.Group("/chat")
		chatApi.Use(middleware.JWTMiddleware())
		chatApi.GET("/ws", chatHandler.Connect)
		chatApi.GET("/convs", chatHandler.GetUserConverse)
		chatApi.POST("/sync", chatHandler.SyncConverse)
	}
}
