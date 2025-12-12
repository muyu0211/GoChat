package service

import (
	"GoChat/internel/repository/cache"
	ws "GoChat/internel/websocket"
	"GoChat/pkg/util"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"log"
	"time"
)

type PushPayLoad struct {
	Msg          []byte `json:"msg_data"`
	ReceiverID   uint64 `json:"receiver_id"`
	TargetServer string `json:"target_server"`
}

type PushService struct {
	redisCache  cache.ICacheRepository
	userService IUserService
}

func NewPushService(rc cache.ICacheRepository, us IUserService) *PushService {
	return &PushService{
		redisCache:  rc,
		userService: us,
	}
}

// Push 推送服务 【对外接口】
func (ps *PushService) Push(ctx context.Context, msg *ws.ReplyMsg) error {
	receiverID := msg.ReceiverID
	// 1. 查询接收方在哪一台服务器
	serveID, err := ps.userService.GetUserLocation(ctx, receiverID)
	if errors.Is(err, redis.Nil) {
		// TODO: 对方不在线, 存入rocketMQ，当用户上线时，由用户主动拉取信息
		log.Printf("用户: %d 不在线", receiverID)
		return ErrUserOffline
	}
	if err != nil {
		return err
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		zap.L().Error("ACK消息序列化失败", zap.Error(err))
		return ErrMarshalJSON
	}
	if serveID == util.ServerID {
		return ps.pushLocal(ctx, receiverID, msgBytes)
	} else {
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
		case <-time.After(100 * time.Millisecond):
			return bufio.ErrBufferFull
		}
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
	if err = ps.publishWithRetry(ctx, util.PubSubChannel, payLoadBytes); err != nil {
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
