package service

import (
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	"GoChat/pkg/util"
	"context"
	"fmt"
	"log"
	"time"

	"go.uber.org/zap"
)

/**
 * 对消息生成全局递增的seqID
 */

type ISeqFactoryService interface {
	GetNextSeqID(ctx context.Context, conversationID string) (uint64, error)
	CheckAndSetDedup(ctx context.Context, conversationID string, clientMsgID string) (bool, error)
	CheckAndSetDedupWithSeq(ctx context.Context, userID uint64, conversationID string, clientMsgID string, expire time.Duration) (bool, uint64, error)
}

type SeqFactoryService struct {
	chatCache cache.ICacheRepository
	convRepo  repository.IConversationRepo
}

func NewSeqFactory(cc cache.ICacheRepository, cr repository.IConversationRepo) ISeqFactoryService {
	return &SeqFactoryService{
		chatCache: cc,
		convRepo:  cr,
	}
}

// GetNextSeqID 获取当前会话的下一个seqID
func (sf *SeqFactoryService) GetNextSeqID(ctx context.Context, conversationID string) (uint64, error) {
	key := fmt.Sprintf("im:seq:%s", conversationID)
	seqID, err := sf.chatCache.Incr(ctx, key)
	if err != nil {
		return 0, ErrServerNotAvailable
	}
	return uint64(seqID), nil
}

// CheckAndSetDedup 检查重复并设置去重键: 使用redis SETNX 操作， 对每个对话进行重复性检查
func (sf *SeqFactoryService) CheckAndSetDedup(ctx context.Context, conversationID string, clientMsgID string) (bool, error) {
	key := fmt.Sprintf("im:dedup:%s:%s", conversationID, clientMsgID)
	// 检查当前会话下，由[clientMsgID]发送来的的消息，是否已经被服务端接收
	suc, err := sf.chatCache.SetNX(ctx, key, clientMsgID, 30*time.Second)
	if err != nil {
		return false, err
	}
	// success 为 true 表示设置成功，说明不是重复消息; 为 false 表示 Key 已存在，说明是重复消息
	return !suc, nil
}

// CheckAndSetDedupWithSeq 事务化：检查消息幂等性和取号（SeqID）
func (sf *SeqFactoryService) CheckAndSetDedupWithSeq(
	ctx context.Context,
	userID uint64,
	conversationID string,
	clientMsgID string,
	dupExpire time.Duration) (duplicated bool, seq uint64, err error) {

	dupKey := util.GetRedisDupKey(conversationID, clientMsgID)
	seqKey := util.GetRedisSeqKey(conversationID)

	// 1. 执行去重和取号
	ret, err := sf.chatCache.DedupAndSeq(ctx, []string{dupKey, seqKey}, dupExpire)
	if err != nil {
		return true, 0, fmt.Errorf("lua script run failed: %w", err)
	}
	switch ret {
	case 0: // 重复消息
		return true, 0, nil
	case -1: // 可能需要初始化 seqID 或从数据库中获取seqID
		var seqID uint64
		if seqID, err = sf.initSeqIfNeeded(ctx, userID, conversationID, seqKey); err != nil {
			return false, 0, fmt.Errorf("init seqID failed: %w", err)
		}
		log.Printf("初始化序号seq：%d, err: %v", seqID, err)
		return false, seqID, nil
	default:
		return false, uint64(ret), nil
	}
}

func (sf *SeqFactoryService) initSeqIfNeeded(
	ctx context.Context,
	userID uint64,
	conversationID string,
	seqKey string,
) (uint64, error) {

	// 1. 尝试获取初始化锁（5 秒）
	lockKey := util.GetRedisSeqLockKey(conversationID)
	ok, err := sf.chatCache.SetNX(ctx, lockKey, 1, 5*time.Second)
	if err != nil {
		return 0, err
	}

	if !ok {
		// 2. 其他 goroutine 正在初始化，等待一下
		time.Sleep(20 * time.Millisecond)
		return 0, nil
	}

	// 3. double check：防止重复初始化
	exist, err := sf.chatCache.Exists(ctx, seqKey)
	if err != nil {
		return 0, err
	}
	if exist {
		return 0, nil
	}

	// 4. 从 DB 读取 lastSeq
	lastSeq, err := sf.convRepo.GetLastSeqID(ctx, userID, conversationID)
	if err != nil || lastSeq == 0 {
		// 新会话兜底
		zap.L().Error("GetLastSeqID failed", zap.Error(err))
		lastSeq = 1
	}

	// 5. 初始化 seqKey（不设置 TTL）
	return lastSeq, sf.chatCache.Set(ctx, seqKey, lastSeq, 0)
}
