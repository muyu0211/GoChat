package router

import (
	"GoChat/internel/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.Engine) {
	// 从配置中构建限流策略
	//globalIPLimit, err := middleware.BuildLimiter("rate_limit.global_ip")
	//if err != nil {
	//	zap.L().Fatal("Failed to build global IP rate limit", zap.Error(err))
	//}

	r.Use(
		middleware.CorsMiddleware(), // 跨域中间件
		middleware.GinLogger(),      // 日志中间件
		middleware.GinRecovery())    // 错误恢复中间件
	//middleware.RateLimiterMiddleware(db.GetRDB(), middleware.KeyByIP, globalIPLimit)) // 限流中间件

	// 注册路由
	RegisterUserRouter(r)
	RegisterWsRouter(r)

	//RegisterBoardRouter(r)
}
