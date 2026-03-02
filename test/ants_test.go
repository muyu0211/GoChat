package test

import (
	"fmt"
	"github.com/panjf2000/ants/v2"
	"sync"
	"testing"
	"time"
)

func TestGos(t *testing.T) {
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			t.Log(i)
		}(i)
	}
	wg.Wait()
	fmt.Println("耗时：", time.Since(start))

}

func TestAnts(t *testing.T) {
	pool, err := ants.NewPool(30)
	if err != nil || pool == nil {
		t.Log("初始化协程池出错")
	}
	defer pool.Release() // 测试结束后关闭协程池，释放资源

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		taskID := i
		_ = pool.Submit(func() {
			defer wg.Done()
			t.Log(taskID)
		})
	}
	wg.Wait()
	t.Log("协程池耗时：", time.Since(start))
}
