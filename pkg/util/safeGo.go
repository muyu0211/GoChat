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
	RetryInterval = 50 * time.Millisecond
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
		retryDelay(i, interval)
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

	return fmt.Errorf("方法: %s重试%d次后失败：%w", fullName, maxTimes, err)
}

func retryDelay(attempt int, interval time.Duration) {
	b := 1
	for range attempt {
		b *= 2
	}
	delay := interval * time.Duration(b)
	time.Sleep(delay)
}
