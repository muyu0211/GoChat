package main

import (
	"GoChat/config"
	"GoChat/pkg/auth"
	"GoChat/pkg/db"
	"GoChat/pkg/logger"
	"GoChat/pkg/util"
	"GoChat/router"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"reflect"
	"runtime/debug"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	_ "net/http/pprof"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var ptr unsafe.Pointer

type appConfig struct {
	Name string `mapstructure:"name"`
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

func startServer() {
	var appCfg *appConfig
	if err := viper.UnmarshalKey("app", &appCfg); err != nil {
		panic(fmt.Errorf("unable to decode into %s, %v", reflect.TypeOf(appCfg).Name(), err))
	}
	atomic.StorePointer(&ptr, unsafe.Pointer(appCfg))
}

//func config() *appConfig {
//	return (*appConfig)(atomic.LoadPointer(&ptr))
//}

func init() {

}

func main() {
	// 启动一个独立的 goroutine 监听 pprof 端口
	//go func() {
	//	log.Println(http.ListenAndServe("localhost:6060", nil))
	//}()

	// 初始化配置文件
	cfg := config.LoadConfig()

	// 服务启动
	//startServer()
	logger.StartLogger(cfg)
	db.StartMySQL(cfg)
	db.StartRedis(cfg)
	auth.StartJWT(cfg)
	util.StartAntsPool(cfg)

	r := gin.New()
	router.InitRouter(r)
	if err := r.Run(cfg.BasicConfig.Port); err != nil {
		zap.L().Fatal("Error: server start error:", zap.Error(err))
	}

	server := &http.Server{
		Addr:    cfg.BasicConfig.Port,
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer stop()

	util.SafeGo(func() {
		zap.L().Info("HTTP server starting", zap.String("addr", server.Addr))
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
