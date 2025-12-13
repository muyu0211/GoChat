package service

import (
	"GoChat/internel/repository/cache"
	"GoChat/pkg/util"
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"time"
)

/**
 * 对消息生成全局递增的seqID
 */

type SeqFactory struct {
	ChatCache cache.ICacheRepository
}

func NewSeqFactory(cc cache.ICacheRepository) *SeqFactory {
	return &SeqFactory{
		ChatCache: cc,
	}
}

// GetNextSeqID 获取当前会话的下一个seqID
func (sf *SeqFactory) GetNextSeqID(ctx context.Context, conversationID string) (uint64, error) {
	key := fmt.Sprintf("im:seq:%s", conversationID)
	seqID, err := sf.ChatCache.Incr(ctx, key)
	if err != nil {
		return 0, ErrServerNotAvailable
	}
	return uint64(seqID), nil
}

// CheckAndSetDedup 检查重复并设置去重键: 使用redis SETNX 操作， 对每个对话进行重复性检查
func (sf *SeqFactory) CheckAndSetDedup(ctx context.Context, conversationID string, clientMsgID string) (bool, error) {
	key := fmt.Sprintf("im:dedup:%s:%s", conversationID, clientMsgID)
	// 检查当前会话下，由[clientMsgID]发送来的的消息，是否已经被服务端接收
	suc, err := sf.ChatCache.SetNX(ctx, key, clientMsgID, 30*time.Second)
	if err != nil {
		return false, err
	}
	// success 为 true 表示设置成功，说明不是重复消息; 为 false 表示 Key 已存在，说明是重复消息
	return !suc, nil
}

// CheckAndSetDedupWithSeq 事务化：检查消息幂等性和取号（SeqID）
func (sf *SeqFactory) CheckAndSetDedupWithSeq(ctx context.Context, conversationID string, clientMsgID string, expire time.Duration) (bool, uint64, error) {
	dupKey := fmt.Sprintf("%s:%s:%s", util.RedisDupKey, conversationID, clientMsgID)
	seqKey := fmt.Sprintf("%s:%s", util.RedisSeqKey, conversationID)

	// Redis事务：MULTI/EXEC
	pipe := sf.ChatCache.TxPipeline()

	pipe.Exists(ctx, dupKey)
	pipe.Incr(ctx, seqKey)
	pipe.SetEX(ctx, dupKey, 1, expire)

	// 执行事务
	cmder, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("redis exec failed: %w", err)
	}

	// 4. 解析结果(事务会返回每个命令的执行结果)
	if exist := cmder[0].(*redis.IntCmd).Val(); exist == 1 {
		return false, 0, nil
	}
	seqID := cmder[1].(*redis.IntCmd).Val()

	return false, uint64(seqID), nil
}
