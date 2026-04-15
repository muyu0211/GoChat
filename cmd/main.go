package main

import (
	"GoChat/config"
	"GoChat/internel/handler"
	"GoChat/internel/infrastructure/mq"
	"GoChat/internel/service"
	"GoChat/pkg/auth"
	"GoChat/pkg/db"
	"GoChat/pkg/logger"
	"GoChat/pkg/util"
	"GoChat/router"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type App struct {
	ChatHandler  *handler.ChatHandler
	UserHandler  *handler.UserHandler
	GroupHandler *handler.GroupHandler
	ChatService  service.IChatService
	PushService  service.IPushService
	GroupService service.IGroupService
	AckConsumer  *mq.AckConsumer
	AckProducer  *mq.AckProducer
}

func main() {
	//启动一个独立的 goroutine 监听 pprof 端口
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	// 初始化配置文件
	cfg := config.LoadConfig()
	log.Println("BUILD VERSION: 2026-03-06-01")
	log.Println("APP FILE DIR:", util.GetAppDir())

	// 服务启动
	logger.StartLogger(cfg)
	db.StartMySQL(cfg)
	db.StartRedis(cfg)
	auth.StartJWT(cfg)
	util.StartAntsPool(cfg)
	util.InitIDGenerator()
	util.Init()

	app, err := InitializeApp()
	if err != nil {
		panic(fmt.Sprintf("依赖注入失败: %v", err))
	}

	// 初始化 Gin 路由器
	r := gin.New()
	// 注册业务路由
	router.InitRouter(r, app.UserHandler, app.ChatHandler, app.GroupHandler)
	// 创建标准http.Server（唯一服务实例，用于优雅关闭）
	server := &http.Server{
		Addr:    cfg.BasicConfig.Port,
		Handler: r,
	}
	// 监听系统终止信号，创建优雅关闭上下文
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer stop()

	// 启动 HTTP 服务
	util.SafeGo(func() {
		zap.L().Info("HTTP server starting", zap.String("addr", server.Addr))
		// 启动服务后，该goroutine会同步阻塞在者当前位置
		if err := server.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			zap.L().Error("HTTP server error:", zap.Error(err))
			stop()
		}
	})
	// 定期释放内存
	util.SafeGo(func() {
		ticker := time.NewTicker(time.Minute * 1)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				debug.FreeOSMemory()
			}
		}
	})

	// 阻塞等待服务关闭信号
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		zap.L().Error("server shutdown error", zap.Error(err))
	}

	util.ClosePool()
	db.CloseRedis()
	db.CloseMySQL()
	logger.CloseLogger()
}
