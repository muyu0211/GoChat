package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"time"
)

// CorsMiddleware 跨域中间件
func CorsMiddleware() gin.HandlerFunc {
	mode := viper.GetString("cors.mode")
	if mode == "allow-all" {
		return cors.New(cors.Config{
			AllowAllOrigins:  true,
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
			ExposeHeaders:    []string{"Content-Length", "X-Trace-ID"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		})
	}

	whitelist := viper.GetStringSlice("cors.whitelist")
	return cors.New(cors.Config{
		AllowOrigins:     whitelist,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "X-Trace-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
