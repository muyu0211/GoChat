package lock

import (
	"runtime"
	"sync/atomic"
)

const maxBackOff = 16 // 最大退避指数

type SpinLock struct {
	flag uint32
}

func (l *SpinLock) Lock() {
	backoff := 1
	// CAS 原子操作
	for !atomic.CompareAndSwapUint32(&l.flag, 0, 1) {
		// 失败则一直自旋
		for i := 0; i < backoff; i++ {
			runtime.Gosched() // 让出当前协程时间片，运行态 -> 就绪态
		}
		if backoff < maxBackOff {
			backoff <<= 1
		}
	}
}

func (l *SpinLock) Unlock() {
	atomic.StoreUint32(&l.flag, 0)
}
