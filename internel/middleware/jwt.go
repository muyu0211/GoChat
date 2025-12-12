package middleware

/**
 * @Description: jwt中间件
 */
import (
	"GoChat/pkg/auth"
	"GoChat/pkg/util"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
)

func JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID, _ := c.Get(TraceIDKey)
		// 1. 先尝试从请求头中获取token
		authHeader := c.GetHeader("Authorization")
		tokenStr := auth.ExactToken(authHeader)
		if tokenStr == "" {
			// 2. 没有则从query参数中获取
			tokenStr = c.Query("token")
			//// TODO：开发阶段，token为接收方id
			//c.Set(util.CtxUserIDKey, tokenStr)
			//c.Next()
			//return

			if tokenStr == "" {
				zap.L().Warn("Invalid authorization format, should be 'Bearer <token>'",
					zap.String("trace_id", traceID.(string)),
					zap.String("ip", c.ClientIP()),
					zap.String("path", c.Request.URL.Path))

				c.JSON(http.StatusUnauthorized, util.NewResMsg("0", "Invalid authorization format, should be 'Bearer <token>'", nil))
				c.Abort()
				return
			}
		}

		claims, err := auth.ParseToken(tokenStr)
		if err != nil || claims == nil {
			zap.L().Warn("Invalid authorization token",
				zap.String("trace_id", traceID.(string)),
				//zap.String("ip", c.ClientIP()),
				//zap.String("path", c.Request.URL.Path),
				zap.String("token", tokenStr),
				zap.Error(err))

			c.JSON(http.StatusUnauthorized, util.NewResMsg("0", "服务器繁忙, 请稍后重试", nil))
			c.Abort()
			return
		}

		// 将当前请求的 userID 信息保存到请求的上下文 c
		c.Set(util.CtxUserIDKey, claims.UserID)
		c.Set(util.CtxEmailKey, claims.Email)
		c.Set(util.CtxPhoneKey, claims.Phone)
		c.Set(util.CtxRoleKey, claims.Role)
		c.Next()
	}
}
