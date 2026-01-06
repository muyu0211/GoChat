package service

import (
	"GoChat/internel/repository/cache"
	ws "GoChat/internel/websocket"
	"GoChat/pkg/util"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type IPushService interface {
	Push(context.Context, ws.Msg) error
	Publish(context.Context, uint64, string, []byte) error
	Subscribe(context.Context, string)
}

type PushPayLoad struct {
	Msg          []byte `json:"msg_data"`
	ReceiverID   uint64 `json:"receiver_id"`
	TargetServer string `json:"target_server"`
}

type PushService struct {
	redisCache  *cache.RedisCache
	userService *UserService
}

func NewPushService(rc *cache.RedisCache, us *UserService) *PushService {
	return &PushService{
		redisCache:  rc,
		userService: us,
	}
}

// Push 推送服务: 将消息推送给接收方
func (ps *PushService) Push(ctx context.Context, msg ws.Msg) error {
	receiverID := msg.GetReceiverID()
	// 1. 查询接收方在哪一台服务器
	serveID, err := ps.userService.GetUserLocation(ctx, receiverID)
	if errors.Is(err, redis.Nil) {
		// 对方不在线, 存入redis
		if err = ps.saveOffline(ctx, msg); err != nil {
			return ErrServerNotAvailable
		}
		log.Printf("用户: %d 不在线", receiverID)
		return ErrUserOffline
	}
	if err != nil {
		return err
	}

	msgBytes, err := msg.Serialize()
	if err != nil {
		zap.L().Error("消息序列化失败", zap.Error(err))
		return ErrMarshalJSON
	}
	if serveID == util.ServerID { // 本机推送
		return ps.pushLocal(ctx, receiverID, msgBytes)
	} else { // 跨服务器推送
		return ps.Publish(ctx, receiverID, serveID, msgBytes)
	}
}

// pushLocal 将消息推送给本地的接收方
func (ps *PushService) pushLocal(ctx context.Context, userID uint64, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, util.PushLocalTimeout)
	defer cancel()

	client := ws.Manager.GetClient(userID)
	if client != nil {
		select {
		case client.DataBuffer <- msg:
			return nil
		case <-time.After(100 * time.Millisecond): // 缓冲区满则阻塞等待
			return bufio.ErrBufferFull
		}
	}
	return nil
}

// saveOffline 保存离线消息
func (ps *PushService) saveOffline(ctx context.Context, msg ws.Msg) error {
	zAddCtx, cancel := context.WithTimeout(ctx, util.RedisZAddTimeout)
	defer cancel()

	// key设计： fmt.Sprintf("im:box:%s:%s", receiverID, msg.ConversationID)
	offlineMsgBoxKey := util.GetRedisBoxKey(msg.GetReceiverID(), msg.GetConversationID())
	msgByte, err := msg.Serialize()
	if err != nil {
		zap.L().Error("序列化消息失败", zap.Error(err))
		return ErrMarshalJSON
	}
	if err = ps.redisCache.ZAdd(zAddCtx, offlineMsgBoxKey, float64(msg.GetSeqID()), msgByte, util.RedisOfflineExpire); err != nil {
		zap.L().Error("保存离线消息出错：", zap.Error(err))
		return ErrServerNotAvailable
	}
	return nil
}

// Publish 接收方在其他服务器，则发布订阅至redis
func (ps *PushService) Publish(ctx context.Context, receiverID uint64, serveID string, msgBytes []byte) error {
	// 构造Payload并序列化（处理错误）
	payLoad := &PushPayLoad{
		Msg:          msgBytes,
		ReceiverID:   receiverID,
		TargetServer: serveID,
	}
	payLoadBytes, err := json.Marshal(payLoad)
	if err != nil {
		zap.L().Error("序列化消息失败", zap.Error(err))
		return ErrMarshalJSON
	}

	// 进行消息发布，带有重试机制
	if err = ps.publishWithRetry(ctx, util.GetRedisPubSubChannel(), payLoadBytes); err != nil {
		return ErrServerNotAvailable
	}
	return nil
}

func (ps *PushService) publishWithRetry(ctx context.Context, channel string, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, util.RedisPublishTimeout)
	defer cancel()

	var lastErr error
	for i := 0; i < util.RedisPublishRetryTimes; i++ {
		err := ps.redisCache.Publish(ctx, channel, msg)
		if err == nil {
			return nil
		}
		lastErr = err
		// 错误重试
		time.Sleep(util.RedisPublishRetryInterval)
	}
	return lastErr
}

// Subscribe 启动订阅
func (ps *PushService) Subscribe(ctx context.Context, channel string) {
	sub := ps.redisCache.Subscribe(ctx, channel)
	defer func() {
		_ = sub.Close()
	}()

	ch := sub.Channel() // 使用管道读取消息
	for {
		// 读取到消息判断是否发给本机
		for msg := range ch {
			var payLoad PushPayLoad
			if err := json.Unmarshal([]byte(msg.Payload), &payLoad); err != nil {
				continue
			}
			if payLoad.TargetServer == util.ServerID {
				_ = ps.pushLocal(ctx, payLoad.ReceiverID, payLoad.Msg)
			}
		}
	}
}
