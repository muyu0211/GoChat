package main

import (
	"GoChat/pkg/auth"
	"GoChat/pkg/db"
	lg "GoChat/pkg/logger"
	"GoChat/pkg/util"
	"GoChat/router"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"reflect"
	"runtime/debug"
	"sync/atomic"
	"time"
	"unsafe"
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

func config() *appConfig {
	return (*appConfig)(atomic.LoadPointer(&ptr))
}

func init() {

}

func main() {
	// 初始化配置文件
	util.InitConfig()

	// 服务启动
	startServer()
	lg.StartLogger()
	db.StartMySQL()
	db.StartRedis()
	auth.StartJWT()

	// 服务关闭
	defer func() {
		lg.CloseLogger()
		db.CloseMySQL()
	}()

	// 定期释放内存 (TODO：清理离线用户缓存)
	util.SafeGo(func() {
		for {
			time.Sleep(time.Minute * 1)
			debug.FreeOSMemory()
		}
	})

	zap.L().Info("================= 服务启动成功 =================")
	r := gin.New()
	router.InitRouter(r)
	if err := r.Run(config().Port); err != nil {
		zap.L().Fatal("Error: server start error:", zap.Error(err))
	}
}
