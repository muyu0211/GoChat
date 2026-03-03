package service

import (
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	"GoChat/pkg/util"
	"context"
	"fmt"
	"strconv"
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
	chatCache *cache.RedisCache
	convRepo  *repository.ConvRepo
}

func NewSeqFactory(cc *cache.RedisCache, cr *repository.ConvRepo) *SeqFactoryService {
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
	seqKey := util.GetRedisChatSeqKey(conversationID)

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
		return false, seqID, nil
	default:
		return false, uint64(ret), nil
	}
}

// initSeqIfNeeded 初始化会话的seqID
func (sf *SeqFactoryService) initSeqIfNeeded(
	ctx context.Context,
	userID uint64,
	conversationID string,
	seqKey string,
) (uint64, error) {
	// 0. 获取分布式锁uuid
	requestID := util.GetUUID()

	// 1. 尝试获取分布式初始化锁（5 秒）
	lockKey := util.GetRedisSeqLockKey(conversationID)
	var ok bool
	err := util.Retry(util.RetryMaxTimes, util.RetryInterval, func() error {
		var err error
		ok, err = sf.chatCache.AcquireLock(ctx, lockKey, requestID, 30*time.Second)
		return err
	})
	if err != nil {
		zap.L().Warn("重试获取分布式锁失败", zap.Error(err), zap.String("lockKey", lockKey))
		return 0, err
	}

	// 2. 流程结束释放分布式锁
	defer func() {
		// TODO: 使用lua脚本释放锁
		_, err = sf.chatCache.Unlock(ctx, lockKey, requestID)
		if err != nil {
			zap.L().Warn("释放锁失败", zap.Error(err), zap.String("lockKey: ", lockKey))
		}
	}()

	if !ok {
		// 其他 goroutine 正在初始化，等待一下
		time.Sleep(20 * time.Millisecond)
		return 0, nil
	}

	// 3. double check：检查seqID是否已经写入缓存, 防止重复初始化
	lastSeqStr, exist, err := sf.chatCache.GetWithExists(ctx, seqKey)
	if err != nil {
		return 0, err
	}
	if exist {
		seq, err := strconv.ParseUint(lastSeqStr, 10, 64)
		if err != nil {
			return 0, err
		}
		return seq, nil
	}

	// 4. 从 DB 读取 lastSeq
	lastSeq, err := sf.convRepo.GetLastSeqID(ctx, userID, conversationID)
	if err != nil || lastSeq == 0 {
		// 新会话兜底
		zap.L().Error("GetLastSeqID failed", zap.Error(err))
		lastSeq = 1
	}

	// 5. 把 seqID 放入缓存中（不设置 TTL）
	_ = sf.chatCache.Set(ctx, seqKey, lastSeq, 0)

	return lastSeq, nil
}
