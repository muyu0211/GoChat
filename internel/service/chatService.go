package service

import (
	"GoChat/internel/infrastructure/mq"
	"GoChat/internel/model/dao"
	"GoChat/internel/model/dto"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	ws "GoChat/internel/websocket"
	"GoChat/pkg/util"
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type IChatService interface {
	Run(ctx context.Context)
	HandleChatMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleAckMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleRevokeMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleDeleteMsg(context.Context, *ws.Client, *ws.SendMsg) error
}

type ChatService struct {
	seqFactory  *SeqFactoryService
	pushService *PushService
	chatRepo    *repository.ChatRepo
	converRepo  *repository.ConvRepo
	redisCache  *cache.RedisCache
	tx          *TxManager
	producer    *mq.AckProducer
	consumer    *mq.AckConsumer

	// 内存缓冲区: Key: "ConversationID" (聚合维度)；Value: MaxSeqID (聚合结果)
	buffer        map[string]uint64
	bufferLock    sync.RWMutex
	batchSize     int
	flushInterval time.Duration
}

func NewChatService(sq *SeqFactoryService, ps *PushService, cr *repository.ChatRepo, ccr *repository.ConvRepo,
	tx *TxManager, rc *cache.RedisCache, producer *mq.AckProducer, consumer *mq.AckConsumer) *ChatService {
	cs := &ChatService{
		seqFactory:    sq,
		pushService:   ps,
		chatRepo:      cr,
		converRepo:    ccr,
		tx:            tx,
		redisCache:    rc,
		producer:      producer,
		consumer:      consumer,
		buffer:        make(map[string]uint64),
		flushInterval: 5 * time.Second,
	}

	// 注册ack消息处理函数, 注意：handler 必须是并发安全的
	if consumer != nil {
		consumer.RegisterHandler(cs.handlerAckFromMq)
	}
	return cs
}

// Run 启动kafka消费服务
func (c *ChatService) Run(ctx context.Context) {
	// 1. 启动定时刷库协程
	util.SafeGo(func() {
		c.flushLoop(ctx)
	})
	if c.consumer != nil {
		// 2. 启动消费者 (Consumer - 阻塞运行)
		c.consumer.Consume(ctx)
	}
}

// HandleChatMsg 处理上行聊天消息(用户发送过来的消息): 整个“上行”流程：去重 -> 取号 -> 落库 -> (给客户端)返回 ACK 数据。
func (c *ChatService) HandleChatMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	var conversationID string
	senderID := client.UserID
	// 1. 获取会话ID
	if req.ConversationID != "" {
		conversationID = req.ConversationID
	} else {
		conversationID = util.GetConversationID(req.SenderID, req.ReceiverID)
	}
	// 2. 幂等性检查 和 生成会话ID （redis事务实现）
	isDup, seqID, err := c.seqFactory.CheckAndSetDedupWithSeq(ctx, senderID, conversationID, req.ClientMsgID, util.RedisDupExpire)
	if err != nil {
		zap.L().Error("消息去重失/取号失败", zap.Error(err))
	}
	if isDup {
		log.Println("消息重复")
		return nil
	}

	createdAt := time.Now().UTC()
	// 3. 消息落库（事务操作：message表新增记录 + conversation表更新相应字段）
	err = c.tx.ExecTx(ctx, func(ctx context.Context) error {
		// 创建 message 表记录
		err := c.chatRepo.Create(ctx, &dao.MessageModel{
			ConversationID: conversationID,
			SeqID:          seqID,
			SenderID:       senderID,
			ReceiverID:     req.ReceiverID,
			Content:        req.Content,
			MsgType:        req.MsgType,
			MsgStatus:      util.MsgStatusRead,
			CreatedAt:      createdAt,
		})

		// 更新 conversation 表 发送方记录
		err = c.converRepo.UpdateSenderConversation(ctx, req.SenderID, req.ReceiverID, conversationID, seqID, createdAt)

		// 更新 conversation 表 接收方记录
		err = c.converRepo.UpdateReceiverConversation(ctx, req.SenderID, req.ReceiverID, conversationID, seqID, createdAt)

		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		zap.L().Error("消息落库失败", zap.Error(err))
		return err
	}

	// 4. 异步更新redis中双方会话表缓存(Set存储)
	util.SafeGo(func() {
		senderConvKey := util.GetRedisConvKey(req.SenderID)
		receiverConvKey := util.GetRedisConvKey(req.ReceiverID)
		pipe := c.redisCache.TxPipeline()
		pipe.SAdd(ctx, senderConvKey, req.ConversationID)
		pipe.SAdd(ctx, receiverConvKey, req.ConversationID)
		_, err = pipe.Exec(ctx)
		if err != nil {
			zap.L().Error("更新 redis 会话表缓存失败", zap.Error(err))
		}
	})

	// 5. 异步调用下行推送服务（带重试+死信队列）
	util.SafeGo(func() {
		reply := &ws.ReplyMsg{
			Cmd:            ws.CmdChat,
			ConversationID: conversationID,
			ClientMsgID:    req.ClientMsgID,
			SeqID:          seqID,
			SenderID:       senderID,
			ReceiverID:     req.ReceiverID,
			Content:        req.Content,
			TimeStamp:      createdAt.UnixMilli(),
		}
		c.pushMsg(ctx, reply)
	})

	// 6. 异步回复ACK（发送方）（主流程不阻塞，失败后重试）
	util.SafeGo(func() {
		ack := &ws.ReplyMsg{
			Cmd:            ws.CmdAck,
			ConversationID: conversationID,
			ReceiverID:     req.SenderID,
			ClientMsgID:    req.ClientMsgID,
			SeqID:          seqID,
			TimeStamp:      createdAt.UnixMilli(),
		}
		c.pushMsg(ctx, ack)
	})

	return nil
}

// pushMsg 通用推送消息方法：将消息推送给客户端
func (c *ChatService) pushMsg(ctx context.Context, reply *ws.ReplyMsg) {
	pushCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := c.pushService.Push(pushCtx, reply); err != nil {
		zap.L().Error("消息推送失败",
			zap.Uint64("sender_id", reply.SenderID),
			zap.String("client_msg_id", reply.ClientMsgID),
			zap.Error(err))
		// TODO: 添加死信队列
	}
}

// HandleAckMsg 将ack消息转发给kafka
func (c *ChatService) HandleAckMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	// 1.构造kafka消息
	event := mq.AckEvent{
		SenderID:       req.SenderID,
		ConversationID: req.ConversationID,
		AckID:          req.AckID,
		TimeStamp:      req.TimeStamp,
		Content:        req.Content,
	}
	msgBytes, err := event.Serialize()
	if err != nil {
		zap.L().Error("序列化消息失败", zap.Error(err))
		return err
	}
	// Key 设计：使用 SenderID 和 ConversationID 保证同一个会话的 ACK 进入同一个 Partition，保证有序性
	key := fmt.Sprintf("%s:%s", strconv.FormatUint(req.SenderID, 10), req.ConversationID)
	if err = c.producer.Publish(ctx, []byte(key), msgBytes); err != nil {
		zap.L().Error("[Kafka] 发送消息失败", zap.Error(err))
	}
	return nil
}

// handlerAckFromMq Kafka回调函数：异步处理kafka发送过来的消息
func (c *ChatService) handlerAckFromMq(ctx context.Context, key, value []byte) error {
	// 1. 接收到ack消息后，存入缓存中，之后立马返回，由另一个协程进行批量处理
	var event = &mq.AckEvent{}
	if err := event.Deserialize(value); err != nil {
		zap.L().Error("[Kafka] 反序列化失败", zap.Error(err))
		return nil // 返回 nil 跳过坏消息，不要卡死消费者
	}

	c.bufferLock.Lock()
	defer c.bufferLock.Unlock()

	keyStr := string(key)
	if maxAckId, ok := c.buffer[keyStr]; ok {
		if event.AckID > maxAckId {
			c.buffer[keyStr] = event.AckID
		}
	} else {
		c.buffer[keyStr] = event.AckID
	}
	return nil
}

// flushBuffer 将缓存中的数据刷入数据库
func (c *ChatService) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.flushToDB()
			return
		case <-ticker.C:
			c.flushToDB()
		}
	}
}

func (c *ChatService) flushToDB() {
	// 核心技巧：将当前 buffer 赋值给临时变量，并创建一个新的空 map 给业务继续用 （写时复制Copy on Write）
	c.bufferLock.Lock()
	if len(c.buffer) == 0 {
		c.bufferLock.Unlock()
		return
	}
	pendingMap := c.buffer
	c.buffer = make(map[string]uint64)
	c.bufferLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// map 转 slice
	updates := make([]*dto.UpdatesAck, 0, len(pendingMap))
	for key, lastAckId := range pendingMap {
		userIDStr, convID, found := strings.Cut(key, ":")
		if !found {
			continue
		}
		uid, _ := strconv.ParseUint(userIDStr, 10, 64)
		updates = append(updates, &dto.UpdatesAck{
			UserID:         uid,
			ConversationID: convID,
			LastAckID:      lastAckId,
		})
	}

	// 对slice 进行全局排序
	sort.Slice(updates, func(i, j int) bool {
		if updates[i].UserID != updates[j].UserID {
			return updates[i].UserID < updates[j].UserID
		}
		return updates[i].ConversationID < updates[j].ConversationID
	})

	// 并发批次处理
	batchSize := 500
	wg := &sync.WaitGroup{}
	for i := 0; i < len(updates); i += batchSize {
		end := i + batchSize
		if end > len(updates) {
			end = len(updates)
		}
		batch := updates[i:end] // 切分批次

		if err := util.SubmitTaskWithContext(ctx, wg, func(ctx context.Context) {
			c.processBatchAck(ctx, batch)
		}); err != nil {
			zap.L().Error("[Redis/MySQL] 批量处理失败", zap.Error(err))
		}
	}

	// 等待所有批次处理完成
	wg.Wait()
}

// processBatchAck 批量处理ack消息
func (c *ChatService) processBatchAck(ctx context.Context, batch []*dto.UpdatesAck) {
	if len(batch) == 0 {
		return
	}

	pipe := c.redisCache.TxPipeline()

	for _, update := range batch {
		offlineMsgBoxKey := util.GetRedisBoxKey(update.UserID, update.ConversationID)
		pipe.ZRemRangeByScore(ctx, offlineMsgBoxKey, "-inf", strconv.FormatUint(update.LastAckID, 10))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		zap.L().Error("[Redis] Pipeline删除消息失败", zap.Error(err))
	}

	if err := c.converRepo.BulkUpdateLastAck(ctx, batch); err != nil {
		zap.L().Error("[MySQL] Batch update failed",
			zap.Uint64("start_user_id", batch[0].UserID),
			zap.Int("batch_size", len(batch)),
			zap.Error(err))
	}
}

// HandleRevokeMsg 处理撤回消息
func (c *ChatService) HandleRevokeMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	// 1. 撤回发送给在线用户的消息；2. 撤回发送给离线用户的消息
	return nil
}

// HandleDeleteMsg 处理删除消息
func (c *ChatService) HandleDeleteMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	return nil
}
