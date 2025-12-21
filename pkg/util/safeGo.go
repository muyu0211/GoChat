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

func Retry(maxTimes int, interval time.Duration, fn func() error) error {
	var err error
	for i := 0; i < maxTimes; i++ {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(interval)
	}
	// 2. 获取函数的PC值
	pc := reflect.ValueOf(fn).Pointer()
	// 3. 通过PC值获取函数信息
	funcInfo := runtime.FuncForPC(pc)
	var fullName string
	if funcInfo == nil {
		fullName = "未知函数"
	} else {
		fullName = funcInfo.Name() // 完整名称：包名.函数名（如main.add）
	}
	return fmt.Errorf("方法: %s重试%d次后失败：%w", fullName, maxTimes, err)
}
