package service

import (
	"GoChat/internel/model/dao"
	"GoChat/internel/model/dto"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	ws "GoChat/internel/websocket"
	"GoChat/pkg/util"
	"context"
	"log"

	"go.uber.org/zap"
)

/**
 * @Description: 用户上线后的消息同步
 */

type ISyncService interface {
	Sync(ctx context.Context, userID uint64)
	GetSessions(ctx context.Context, userID uint64) (*dto.UserSession, error)
	SyncConverse(ctx context.Context, userID uint64, conversationID string, seqID uint64, limit int64) ([]ws.ReplyMsg, bool, error)
}

type SyncService struct {
	redisCache *cache.RedisCache
	chatRepo   *repository.ChatRepo
	convRepo   *repository.ConvRepo
}

func NewSyncService(rc *cache.RedisCache, cr *repository.ChatRepo, ccr *repository.ConvRepo) *SyncService {
	return &SyncService{
		redisCache: rc,
		chatRepo:   cr,
		convRepo:   ccr,
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
	// 1. 查询redis中缓存的当前用户的会话列表
	convKey := util.GetRedisConvKey(userID)
	conversationIDs, exist, redisErr := ss.redisCache.SMembersWithCheck(ctx, convKey)

	if redisErr != nil {
		zap.L().Warn("查询Redis缓存出错，降级到数据库查询",
			zap.Error(redisErr),
			zap.Uint64("userID", userID))
	}

	var conversations []dao.ConversationModel
	var sqlErr error
	var shouldDB = false

	// 是否进行数据库查询 convIDs
	if !exist || redisErr != nil || len(conversationIDs) == 0 {
		shouldDB = true
	}

	if !shouldDB {
		conversations, sqlErr = ss.convRepo.GetConvByUserIDConvID(ctx, userID, conversationIDs)
		if sqlErr != nil || len(conversations) != len(conversationIDs) {
			shouldDB = true
			zap.L().Warn("缓存数据不完整，回退到数据库查询",
				zap.Int("cachedCount", len(conversationIDs)),
				zap.Int("dbCount", len(conversations)),
				zap.Error(sqlErr))
		}
	}

	if shouldDB {
		conversations, sqlErr = ss.convRepo.GetConvByUserID(ctx, userID)
		if sqlErr != nil {
			zap.L().Error("查询用户会话失败",
				zap.Error(sqlErr),
				zap.Uint64("userID", userID))
			return nil, ErrServerNotAvailable
		}
	}

	// 异步进行缓存更新（无论是否命中都应该更新缓存）
	util.SafeGo(func() {
		ctxAsync := context.Background()
		if err := ss.updateConvCache(ctxAsync, userID, conversations); err != nil {
			zap.L().Warn("异步更新会话缓存失败",
				zap.Error(err),
				zap.Uint64("userID", userID))
		}
	})

	// 处理空数据
	if conversations == nil {
		return &dto.UserSession{
			UserID: userID,
			Convs:  make([]dto.Conversations, 0),
		}, nil
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

// updateConvCache 更新会话缓存数据
func (ss *SyncService) updateConvCache(ctx context.Context, userID uint64, conversations []dao.ConversationModel) error {
	if conversations == nil || len(conversations) == 0 {
		return nil
	}
	convKey := util.GetRedisConvKey(userID)
	// 用户的会话列表由Set进行存储，convKey=key + userID
	convIDs := make([]string, 0, len(conversations))
	for _, conv := range conversations {
		convIDs = append(convIDs, conv.ConversationID)
	}
	if err := ss.redisCache.UpdateUserConv(ctx, convKey, 0, convIDs); err != nil {
		zap.L().Error("更新用户会话缓存失败",
			zap.Error(err),
		)
		return err
	}
	return nil
}

// SyncConverse 同步会话内的消息
func (ss *SyncService) SyncConverse(ctx context.Context, userID uint64, conversationID string, seqID uint64, limit int64) ([]ws.ReplyMsg, bool, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}

	// 初始化返回切片（避免返回 nil）
	replyMsgs := make([]ws.ReplyMsg, 0)

	// 1. 查找redis中是否缓存了离线消息
	offlineMsgBoxKey := util.GetRedisBoxKey(userID, conversationID)
	msgBytes, exist, err := ss.redisCache.ZRange(ctx, offlineMsgBoxKey, seqID+1, limit)

	var hitCache = true
	if err != nil || !exist || len(msgBytes) == 0 {
		hitCache = false
	}

	if hitCache { // 命中缓存
		cacheMsgs := make([]ws.ReplyMsg, 0, len(msgBytes))
		for _, msgByte := range msgBytes {
			var msg ws.ReplyMsg
			if err = msg.Deserialize(msgByte); err == nil {
				cacheMsgs = append(cacheMsgs, msg)
			}
		}
		if len(cacheMsgs) > 0 {
			// 完美命中缓存, 直接返回数据
			if cacheMsgs[0].SeqID == seqID+1 {
				return cacheMsgs, int64(len(cacheMsgs)) == limit, nil
			}
		}
		hitCache = false
	}

	// 2. 缓存未命中则查库
	if !hitCache {
		dbMsgs, err := ss.chatRepo.GetMsgsByLastSeqID(ctx, userID, conversationID, seqID, limit)
		if err != nil {
			zap.L().Error("查找数据库出错:", zap.Error(err))
			return nil, false, err
		}

		for _, msg := range dbMsgs {
			replyMsgs = append(replyMsgs, ws.ReplyMsg{
				Cmd:            ws.CmdSingleChat,
				ConversationID: msg.ConversationID,
				Content:        msg.Content,
				SenderID:       msg.SenderID,
				ReceiverID:     msg.ReceiverID,
				SeqID:          msg.SeqID,
				TimeStamp:      msg.CreatedAt.UnixMilli(),
			})
		}
	}
	log.Printf("len replyMsgs: %d", len(replyMsgs))

	return replyMsgs, int64(len(replyMsgs)) == limit, nil
}
