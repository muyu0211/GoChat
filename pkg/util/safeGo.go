package util

import (
	"fmt"
	"reflect"
	"runtime"
	"time"

	"go.uber.org/zap"
)

const (
	RetryMaxTimes = 2
	RetryInterval = 100 * time.Millisecond
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

func SafeGoWithArgs(fn func(args ...interface{}), args ...interface{}) {
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
		fn(args...)
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

	// 重试全部失败后操作
	pc := reflect.ValueOf(fn).Pointer()
	funcInfo := runtime.FuncForPC(pc)
	fullName := ""
	if funcInfo == nil {
		fullName = "未知函数"
	} else {
		fullName = funcInfo.Name()
	}

	zap.L().Warn("方法重试失败", zap.String("方法", fullName), zap.Int("重试次数", maxTimes), zap.Error(err))
	return fmt.Errorf("方法: %s重试%d次后失败：%w", fullName, maxTimes, err)
}
