package util

import (
	"GoChat/config"
	"context"
	"log"
	"sync"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
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
	//go func() {
	//	// 监听系统退出信号
	//	sigChan := make(chan os.Signal, 1)
	//	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	//	<-sigChan
	//
	//	// 关闭协程池，等待所有任务执行完成
	//	log.Println("开始关闭ants协程池...")
	//	AntsPool.Release()
	//	log.Println("ants协程池已关闭")
	//}()
}

func ClosePool() {
	log.Println("开始关闭 ants 协程池...")
	AntsPool.Release()
	log.Println("ants 协程池已关闭")
}

func Submit(task func()) error {
	return AntsPool.Submit(task)
}

func SubmitTaskWithContext(ctx context.Context, wg *sync.WaitGroup, task func(context.Context)) error {
	wg.Add(1)
	if err := AntsPool.Submit(func() {
		defer wg.Done()
		if ctx.Err() != nil {
			return
		}
		task(ctx)
	}); err != nil {
		wg.Done()
		zap.L().Error("[AntsPool] 提交任务失败", zap.Error(err))
		return err
	}
	return nil
}
