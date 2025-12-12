package service

import (
	"GoChat/internel/repository/cache"
	"context"
	"fmt"
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
