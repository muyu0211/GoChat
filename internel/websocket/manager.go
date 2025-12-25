package websocket

import (
	"GoChat/pkg/util"
	"log"
	"strconv"
	"sync"
)

/*
*
复杂管理全部的websocket连接
*/
var bucketCount uint32 = 32

type ClientManager struct {
	Buckets []*Bucket
}

type Bucket struct {
	// 使用 UserID 作为 Key 为了方便查找用户是否在线
	Clients map[interface{}]*Client
	sync.RWMutex
}

var Manager *ClientManager

func init() {
	Manager = &ClientManager{
		Buckets: make([]*Bucket, bucketCount),
	}
	// 初始化bucket
	var i uint32
	for i = 0; i < bucketCount; i++ {
		Manager.Buckets[i] = &Bucket{
			Clients: make(map[interface{}]*Client),
		}
	}
}

func (m *ClientManager) getBucketIndex(userID uint64) uint32 {
	// 使用 FNV-1a 哈希算法，速度快，碰撞率低
	hashed := util.GetHashed(strconv.FormatUint(userID, 10))
	return hashed % bucketCount
}

// GetBucket 根据 UserID 获取对应的 Bucket
func (m *ClientManager) GetBucket(userID uint64) *Bucket {
	idx := m.getBucketIndex(userID)
	return m.Buckets[idx]
}

// Register 注册连接
func (m *ClientManager) Register(client *Client, userID uint64) {
	// 对userID计算hash，取模后获取bucket索引
	bucket := m.GetBucket(userID)
	bucket.Lock()
	defer bucket.Unlock()

	// TODO: 发生碰撞如何处理
	if _, ok := bucket.Clients[userID]; ok {
		//client := bucket.Clients[userID]
		//if client != nil {
		//	client.Close()
		//}
	}
	bucket.Clients[userID] = client
}

// Unregister 注销连接
func (m *ClientManager) Unregister(userID uint64) {
	bucket := m.GetBucket(userID)
	bucket.Lock()
	defer bucket.Unlock()

	if _, ok := bucket.Clients[userID]; ok {
		client := bucket.Clients[userID]
		if client != nil {
			// 先关闭管道
			if client.DataBuffer != nil {
				close(client.DataBuffer)
			}
			// 再关闭连接
			client.Close()
		}
		delete(bucket.Clients, userID)
	}
	log.Printf("用户：%d 下线", userID)
}

// GetClient 获取连接
func (m *ClientManager) GetClient(userID uint64) *Client {
	bucket := m.GetBucket(userID)
	bucket.RLock()
	defer bucket.RUnlock()
	return bucket.Clients[userID]
}

// GetAllClient 获取所有连接 TODO：测试使用
func (m *ClientManager) GetAllClient() []uint64 {
	var wg sync.WaitGroup
	var l sync.Mutex
	clients := make([]uint64, 0)

	for i := 0; i < len(m.Buckets); i++ {
		bucket := m.Buckets[i]
		wg.Add(1) // 1. 计数加 1

		// 注意：这里最好直接用 go func，如果是 util.SafeGo 需要确保它能兼容 WaitGroup
		go func(b *Bucket) {
			defer wg.Done() // 3. 任务结束计数减 1

			b.RLock()
			// 优化：先复制出一份数据或者收集到局部切片，减少对全局锁 l 的占用时间
			localIDs := make([]uint64, 0, len(b.Clients))
			for _, client := range b.Clients {
				localIDs = append(localIDs, client.UserID)
			}
			b.RUnlock()

			// 统一合并，减少锁粒度
			l.Lock()
			clients = append(clients, localIDs...)
			l.Unlock()
		}(bucket)
	}

	wg.Wait()
	return clients
}
