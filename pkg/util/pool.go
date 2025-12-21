package util

import (
	"GoChat/config"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/panjf2000/ants/v2"
)

var AntsPool *ants.Pool

func StartAntsPool(cfg *config.Config) {
	antsCfg := cfg.AntsConfig
	var err error
	AntsPool, err = ants.NewPool(antsCfg.PoolSize, ants.WithExpiryDuration(antsCfg.ExpiryDuration))
	if err != nil {
		panic(err)
	}
	// 优雅关闭协程池：监听程序退出信号（如Ctrl+C、kill命令）
	go func() {
		// 监听系统退出信号
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		<-sigChan

		// 关闭协程池，等待所有任务执行完成
		log.Println("开始关闭ants协程池...")
		AntsPool.Release()
		log.Println("ants协程池已关闭")
	}()
}

func Submit(task func()) error {
	return AntsPool.Submit(task)
}

func SubmitTaskWithContext(ctx context.Context, task func()) error {
	wrappedTask := func() {
		select {
		case <-ctx.Done():
			return
		default:
			task()
		}
	}
	return AntsPool.Submit(wrappedTask)
}
