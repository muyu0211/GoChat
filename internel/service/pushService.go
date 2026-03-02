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
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
)

type IPushService interface {
	Push(context.Context, []ws.Msg, uint64) error
	BatchPush(ctx context.Context, msgs []ws.Msg, receiversID []uint64) error
	Publish(context.Context, uint64, string, []byte) error
	Subscribe(ctx context.Context, channel string)
}

type PushPayLoad struct {
	Msg          []byte `json:"msg_data"`
	ReceiverID   uint64 `json:"receiver_id"`
	TargetServer string `json:"target_server"`
}

type PushService struct {
	pushPool    *ants.Pool // 协程池
	redisCache  *cache.RedisCache
	userService *UserService
}

func NewPushService(rc *cache.RedisCache, us *UserService) *PushService {
	pool, err := ants.NewPool(100, ants.WithExpiryDuration(30*time.Second))
	if err != nil {
		panic("创建推送服务程池失败")
	}

	ps := &PushService{
		pushPool:    pool,
		redisCache:  rc,
		userService: us,
	}

	// 启动redis订阅
	util.SafeGo(func() {
		log.Println("启动redis 订阅")
		ps.Subscribe(context.Background(), util.GetRedisPubSubChannel())
	})

	return ps
}

// Push 推送服务: 将消息推送给接收方
func (ps *PushService) Push(ctx context.Context, msgs []ws.Msg, receiverID uint64) error {
	// 0. 对消息进行序列化
	msgsBytes := make([][]byte, 0, len(msgs))
	for _, msg := range msgs {
		b, err := msg.Serialize()
		if err != nil {
			zap.L().Error("消息序列化失败", zap.Error(err))
			return ErrMarshalJSON
		}
		msgsBytes = append(msgsBytes, b)
	}

	// 1. 查询接收方在哪一台服务器
	serveID, err := ps.userService.GetUserLocation(ctx, receiverID)
	if errors.Is(err, redis.Nil) {
		// 对方不在线, 存入redis
		for _, msg := range msgs {
			if err = ps.saveOffline(ctx, msg); err != nil {
				return ErrServerNotAvailable
			}
		}
		return ErrUserOffline
	}
	if err != nil {
		return err
	}

	if serveID == util.ServerID { // 本机推送
		return ps.pushLocal(ctx, receiverID, msgs)
	} else { // 跨服务器推送
		for _, msgBytes := range msgsBytes {
			if err := ps.Publish(ctx, receiverID, serveID, msgBytes); err != nil {
				return err
			}
		}
		return nil
	}
}

// BatchPush 批量推送
func (ps *PushService) BatchPush(ctx context.Context, msgs []ws.Msg, receiversIDs []uint64) error {
	if len(msgs) == 0 || len(receiversIDs) == 0 {
		return nil
	}

	// 1. 按服务器对接收者分组
	groupedReceivers, err := ps.groupReceiversByServer(ctx, receiversIDs)
	if err != nil {
		zap.L().Error("分组接收者失败", zap.Error(err))
		return err
	}

	wg := sync.WaitGroup{}
	// 2. 处理本机在线用户
	if localOnlineUsers, ok := groupedReceivers["local_online"]; ok && len(localOnlineUsers) > 0 {
		wg.Add(1)
		ps.pushPool.Submit(func() {
			defer wg.Done()
			ps.batchPushLocal(ctx, msgs, localOnlineUsers)
		})
	}

	// 4. 处理本机离线用户（批量保存离线消息）
	if localOfflineUsers, ok := groupedReceivers["offline"]; ok && len(localOfflineUsers) > 0 {
		wg.Add(1)
		ps.pushPool.Submit(func() {
			defer wg.Done()
			ps.batchSaveOffline(ctx, msgs, localOfflineUsers)
		})
	}

	// 5. 处理其他服务器用户（按服务器批量发布）
	for serverID, serverUsers := range groupedReceivers {
		// 跳过本机在线和离线用户
		if serverID == util.ServerID ||
			serverID == "local_online" ||
			serverID == "offline" ||
			len(serverUsers) == 0 {

			continue
		}

		wg.Add(1)
		sID, users := serverID, serverUsers
		ps.pushPool.Submit(func() {
			defer wg.Done()
			ps.batchPublish(ctx, msgs, sID, users)
		})
	}

	wg.Wait()
	return nil
}

// groupReceiversByServer 按服务器对接收者分组
func (ps *PushService) groupReceiversByServer(ctx context.Context, receiversID []uint64) (map[string][]uint64, error) {
	result := make(map[string][]uint64)
	result["local_online"] = make([]uint64, 0) // 本机在线
	result["offline"] = make([]uint64, 0)      // 离线

	for _, receiverID := range receiversID {
		// 先检查是否本机在线
		client := ws.Manager.GetClient(receiverID)
		if client != nil {
			result["local_online"] = append(result["local_online"], receiverID)
			continue
		}

		// 查询用户位置
		serveID, err := ps.userService.GetUserLocation(ctx, receiverID)
		if errors.Is(err, redis.Nil) {
			// 离线用户
			result["offline"] = append(result["offline"], receiverID)
			continue
		}
		if err != nil {
			zap.L().Error("查询用户位置失败", zap.Uint64("userID", receiverID), zap.Error(err))
			continue
		}

		if serveID == util.ServerID {
			// 本机但不在线（可能刚下线）
			result["offline"] = append(result["offline"], receiverID)
		} else {
			// 其他服务器
			if _, ok := result[serveID]; !ok {
				result[serveID] = make([]uint64, 0)
			}
			result[serveID] = append(result[serveID], receiverID)
		}
	}

	return result, nil
}

// batchPushLocal 批量推送给本机用户
func (ps *PushService) batchPushLocal(ctx context.Context, msgsBytes []ws.Msg, userIDs []uint64) {
	// 替换消息中的接收者

	for _, userID := range userIDs {
		if err := ps.pushLocal(ctx, userID, msgsBytes); err != nil {
			zap.L().Error("批量推送本地用户失败", zap.Uint64("userID", userID), zap.Error(err))
		}
	}
}

// batchSaveOffline 批量保存离线消息
func (ps *PushService) batchSaveOffline(ctx context.Context, msgs []ws.Msg, userIDs []uint64) {
	// TODO：优化为一次redis操作
	for _, userID := range userIDs {
		for _, msgBytes := range msgs {
			if err := ps.saveOffline(ctx, msgBytes); err != nil {
				zap.L().Error("批量保存离线消息失败", zap.Uint64("userID", userID), zap.Error(err))
			}
		}
	}
}

// batchPublish 批量发布到其他服务器
func (ps *PushService) batchPublish(ctx context.Context, msgs []ws.Msg, serverID string, userIDs []uint64) {
	for _, userID := range userIDs {
		for _, msg := range msgs {
			if err := ps.Publish(ctx, userID, serverID, msg); err != nil {
				zap.L().Error("批量发布跨服务器消息失败", zap.Uint64("userID", userID), zap.String("serverID", serverID), zap.Error(err))
			}
		}
	}
}

// pushLocal 将消息推送给本地的接收方
func (ps *PushService) pushLocal(ctx context.Context, userID uint64, msgs []ws.Msg) error {
	ctx, cancel := context.WithTimeout(ctx, util.PushLocalTimeout)
	defer cancel()

	client := ws.Manager.GetClient(userID)
	if client != nil {
		for _, msg := range msgs {
			select {
			case client.DataBuffer <- msg:
				return nil
			case <-time.After(100 * time.Millisecond): // 缓冲区满则阻塞等待
				return bufio.ErrBufferFull
			}
		}
	}
	return nil
}

// saveOffline 保存离线消息
func (ps *PushService) saveOffline(ctx context.Context, msg ws.Msg) error {
	zAddCtx, cancel := context.WithTimeout(ctx, util.RedisZAddTimeout)
	defer cancel()

	log.Printf("saveOffline 保存离线消息, 接收方: %d 群组: %s", msg.GetReceiverID(), msg.GetConversationID())
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
func (ps *PushService) Publish(ctx context.Context, receiverID uint64, serveID string, msg ws.Msg) error {
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
				_ = ps.pushLocal(ctx, payLoad.ReceiverID, [][]byte{payLoad.Msg})
			}
		}
	}
}
