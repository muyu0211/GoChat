package middleware

import (
	"GoChat/pkg/util"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"
)

/**
 * @Description:日志中间件
 */

const TraceIDKey = "X-Trace-ID"

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 为每个请求生成一个唯一的TraceID
		traceID := c.Request.Header.Get(TraceIDKey)
		if traceID == "" {
			traceID = uuid.New().String()
		}
		c.Set(TraceIDKey, traceID)
		c.Header(TraceIDKey, traceID)
		c.Next()

		// 上线场景中，只针对500以上的请求状态进行日志记录
		statusCode := c.Writer.Status()
		var errMsg string
		if err, ok := c.Get("err"); ok && err != nil {
			if e, ok := err.(error); ok {
				errMsg = e.Error()
			} else if e, ok := err.(string); ok {
				errMsg = e
			}
		}
		if statusCode != 200 {
			zap.L().Error(path,
				zap.Int("status", statusCode),
				zap.String("method", c.Request.Method),
				zap.String("path", path),
				zap.String("query", query),
				zap.String("ip", c.ClientIP()),
				zap.String("user-agent", c.Request.UserAgent()),
				zap.String("trace_id", traceID),
				zap.Duration("cost", time.Since(start)),
				zap.String("err", errMsg))
		}
	}
}

func GinRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 获取traceID
				var strTraceID = "unknown"
				if v, exists := c.Get(TraceIDKey); exists {
					if v, ok := v.(string); ok {
						strTraceID = v
					}
				}

				// 2. 获取完整的请求信息用于日志 (可选，帮助排查问题)
				httpRequest, _ := httputil.DumpRequest(c.Request, false)

				// 3. 判断是否是 "Broken Pipe" 错误
				// (网络底层错误，通常不需要打印堆栈，也不需要返回 500，因为连接断了)
				var brokenPipe bool
				var ne *net.OpError
				if errors.As(err.(error), &ne) {
					var se *os.SyscallError
					if errors.As(ne.Err, &se) {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				// 4. 记录日志 (Zap)
				if brokenPipe {
					zap.L().Error(c.Request.URL.Path,
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
					c.Abort()
					return
				}

				// 记录完整的 Panic 堆栈
				zap.L().Error("recovery from panic",
					zap.Any("error", err),
					zap.String("trace_id", strTraceID), // 注意类型断言安全
					//zap.String("request", string(httpRequest)),
					zap.Stack("stacktrace"),
				)

				// 检查请求头，判断是否是http请求，如果是则返回500错误
				if util.IsHTTP(c) {
					// 5. 普通 HTTP 请求：返回 500 JSON
					c.JSON(http.StatusInternalServerError, gin.H{
						"message": "Internal Server Error",
						"error":   err, // 生产环境建议隐藏具体 error
					})
					return
				}

				// 其他请求则直接返回，不写请求体
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}()
		c.Next()
	}
}
