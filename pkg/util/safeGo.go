package util

import (
	"fmt"
	"go.uber.org/zap"
	"time"
)

// SafeGo 封装 goroutine 用于保证子协程的报错安全退出
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				// 记录报错日志
				zap.L().Error("goroutine panic",
					zap.Any("error", err),
					zap.Stack("stack"),
				)
			}
		}()
		fn()
	}()
}

func Retry(maxTimes int, interval time.Duration, fn func() error) error {
	var err error
	for i := 0; i < maxTimes; i++ {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("重试%d次后失败：%w", maxTimes, err)
}
