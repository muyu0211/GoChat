package service

import (
	"GoChat/internel/model/dao"
	"GoChat/internel/model/dto"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	ws "GoChat/internel/websocket"
	"GoChat/pkg/util"
	"context"
	"fmt"
	"go.uber.org/zap"
	"log"
	"strconv"
)

/**
 * @Description: 用户上线后的消息同步
 */

type ISyncService interface {
	Sync(ctx context.Context, userID uint64, seqID uint64)
	SyncSession(ctx context.Context, userID uint64)
	syncConversation(ctx context.Context, conversationID string, seqID uint64, limit int64) (*[]ws.ReplyMsg, bool, error)
}

type SyncService struct {
	redisCache       cache.ICacheRepository
	chatRepo         repository.IChatRepo
	conversationRepo repository.IConversationRepo
}

func NewSyncService(rc cache.ICacheRepository, cr repository.IChatRepo, ccr repository.IConversationRepo) *SyncService {
	return &SyncService{
		redisCache:       rc,
		chatRepo:         cr,
		conversationRepo: ccr,
	}
}

// Sync 同步用户未读消息
func (ss *SyncService) Sync(ctx context.Context, userID uint64) {
	// 1. 先同步单聊的未读消息
	// TODO: 考虑是否需要开启多个协程查找
	// TODO: 后续完善群聊的同步
	log.Printf("用户: %d 消息同步(占位符)", userID)
}

// GetSessions 获取用户会话列表
func (ss *SyncService) GetSessions(ctx context.Context, userID uint64) (*dto.UserSession, error) {
	// 1. 获取用户会话列表先查Redis
	key := fmt.Sprintf("%s:%s", util.RedisSessionKey, strconv.FormatUint(userID, 10))
	// 获取当前用户的会话ID
	conversationIDs, exist, redisErr := ss.redisCache.SMembersWithCheck(ctx, key)

	if redisErr != nil {
		zap.L().Warn("查询Redis缓存出错，降级到数据库查询",
			zap.Error(redisErr),
			zap.Uint64("userID", userID))
	}

	var conversations []dao.Conversation
	var sqlErr error
	if exist && len(conversationIDs) > 0 {
		// 缓存命中
		log.Printf("缓存命中")
		conversations, sqlErr = ss.conversationRepo.GetByConversationIDs(ctx, conversationIDs)
		if sqlErr != nil {
			zap.L().Error("查找数据库出错:", zap.Error(sqlErr))
			// 降级查询
			conversations, sqlErr = ss.conversationRepo.GetByUserID(ctx, userID)
		}
	} else {
		// 缓存未命中或为空，直接从数据库查询
		log.Printf("缓存未命中")
		conversations, sqlErr = ss.conversationRepo.GetByUserID(ctx, userID)
	}

	// 统一的数据库错误处理
	if sqlErr != nil {
		zap.L().Error("查询用户会话失败",
			zap.Error(sqlErr),
			zap.Uint64("userID", userID))
		return nil, ErrServerNotAvailable
	}

	// 进行缓存更新
	if !exist || redisErr != nil {
		util.SafeGo(func() {
			ctxAsync := context.Background()
			if err := ss.updateSessionCache(ctxAsync, userID, conversations); err != nil {
				zap.L().Warn("异步更新会话缓存失败",
					zap.Error(err),
					zap.Uint64("userID", userID))
			}
		})
	}

	// 构建返回数据
	userSessions := &dto.UserSession{
		UserID: userID,
		Convs:  make([]dto.Conversations, 0, len(conversations)),
	}
	for _, conv := range conversations {
		userSessions.Convs = append(userSessions.Convs, dto.Conversations{
			ConversationID: conv.ConversationID,
			LastSeq:        conv.LastSeqID,
			UnreadCount:    conv.UnreadCount,
		})
	}

	return userSessions, nil
}

func (ss *SyncService) updateSessionCache(ctx context.Context, userID uint64, conversations []dao.Conversation) error {
	log.Println("TODO: 更新会话缓存数据")
	return nil
}

func (ss *SyncService) syncConversation(ctx context.Context, conversationID string, seqID uint64, limit int64) (*[]ws.ReplyMsg, bool, error) {
	if limit < 0 {
		limit = 50
	}

	// 1. 先从redis查找
	msgBytes, err := ss.redisCache.ZRange(ctx, conversationID, seqID, limit)
	if err == nil && len(msgBytes) > 0 {
		msgs := make([]ws.ReplyMsg, len(msgBytes))
		// 对消息进行反序列化
		for i, msgByte := range msgBytes {
			if err = msgs[i].Deserialize(msgByte); err != nil {
				msgs[i] = ws.ReplyMsg{Cmd: ws.CmdChat}
				continue
			}
		}
		// 进行消息连续性判断
		if msgs[0].SeqID == seqID+1 {
			hasMore := int64(len(msgs)) == limit
			return &msgs, hasMore, nil
		}
	}

	// 2. 缓存未命中则查库
	msgs, num, err := ss.chatRepo.GetByConversationIDAfterSeqID(ctx, conversationID, seqID, limit)
	if err != nil {
		zap.L().Error("查找数据库出错:", zap.Error(err))
		return nil, false, err
	}
	if num == 0 {
		return nil, false, nil
	}
	replyMsgs := make([]ws.ReplyMsg, num)
	for i, msg := range msgs {
		replyMsgs[i] = ws.ReplyMsg{
			Cmd:            ws.CmdChat,
			ConversationID: msg.ConversationID,
			Content:        msg.Content,
			SenderID:       msg.SenderID,
			ReceiverID:     msg.ReceiverID,
			SeqID:          msg.SeqID,
			TimeStamp:      msg.CreatedAt.UnixMilli(),
		}
	}
	return &replyMsgs, num == limit, nil
}
