package router

import (
	"GoChat/internel/handler"
	"GoChat/internel/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterGroupRouter(r *gin.Engine, groupHandler *handler.GroupHandler) {
	// group 组路由
	{
		groupApi := r.Group("/group")
		groupApi.Use(middleware.JWTMiddleware())
		groupApi.POST("/create", groupHandler.CreateGroup)
	}
}
