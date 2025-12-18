package service

import (
	"GoChat/internel/infrastructure/mq"
	"GoChat/internel/model/dao"
	"GoChat/internel/repository"
	ws "GoChat/internel/websocket"
	"GoChat/pkg/util"
	"context"
	"go.uber.org/zap"
	"log"
	"sync"
	"time"
)

type IChatService interface {
	Run(ctx context.Context)
	HandleChatMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleAckMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleRevokeMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleDeleteMsg(context.Context, *ws.Client, *ws.SendMsg) error
}

type chatService struct {
	seqFactory  ISeqFactoryService
	pushService IPushService
	chatRepo    repository.IChatRepo
	producer    mq.Producer
	consumer    mq.Consumer

	// 内存缓冲区: Key: "ConversationID" (聚合维度)；Value: MaxSeqID (聚合结果)
	buffer        map[string]uint64
	bufferLock    sync.RWMutex
	batchSize     int           // 攒够多少条刷库
	flushInterval time.Duration // 多久刷一次库
}

func NewChatService(sq ISeqFactoryService, ps IPushService, cr repository.IChatRepo, producer mq.Producer, consumer mq.Consumer) IChatService {
	cs := &chatService{
		seqFactory:    sq,
		pushService:   ps,
		chatRepo:      cr,
		producer:      producer,
		consumer:      consumer,
		buffer:        make(map[string]uint64),
		flushInterval: 10 * time.Second,
	}

	// 注册ack消息处理函数, 注意：handler 必须是并发安全的
	if consumer != nil {
		consumer.RegisterHandler(cs.handlerAckFromMq)
	}
	return cs
}

// Run 启动kafka消费服务
func (c *chatService) Run(ctx context.Context) {
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
func (c *chatService) HandleChatMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	// TODO：进行消息参数校验
	senderID := client.UserID
	// 1. 获取会话ID
	conversationID := util.GetConversationID(senderID, req.ReceiverID)
	// 2. 幂等性检查 和 生成会话ID （redis事务实现）
	isDup, seqID, err := c.seqFactory.CheckAndSetDedupWithSeq(ctx, conversationID, req.ClientMsgID, time.Hour)
	if err != nil {
		zap.L().Error("消息去重失/取号失败", zap.Error(err))
	}
	if isDup {
		log.Println("消息重复")
		return nil
	}
	// 3. 消息落库
	createdAt := time.Now().UTC()
	msg := dao.MessageModel{
		ConversationID: conversationID,
		SeqID:          seqID,
		SenderID:       senderID,
		ReceiverID:     req.ReceiverID,
		Content:        req.Content,
		MsgType:        req.MsgType,
		MsgStatus:      util.MsgStatusRead,
		CreatedAt:      createdAt,
	}
	if err = util.Retry(util.RetryMaxTimes, util.RetryInterval, func() error {
		return c.chatRepo.Create(ctx, &msg)
	}); err != nil {
		return err
	}

	// 4. 异步调用下行推送服务（带重试+死信队列）
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

	// 5. 异步回复ACK（发送方）（主流程不阻塞，失败后重试）
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
func (c *chatService) pushMsg(ctx context.Context, reply *ws.ReplyMsg) {
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
func (c *chatService) HandleAckMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	// 1.构造kafka消息
	event := mq.AckEvent{
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
	// Key 设计：使用 ConversationID 保证同一个会话的 ACK 进入同一个 Partition，保证有序性
	key := util.GetConversationID(req.SenderID, req.ReceiverID)
	if err = c.producer.Publish(ctx, mq.KafkaAckTopic, []byte(key), msgBytes); err != nil {
		zap.L().Error("[Kafka] 发送消息失败", zap.Error(err))
	}
	return nil
}

// handlerAckFromMq Kafka回调函数：异步处理kafka发送过来的消息
func (c *chatService) handlerAckFromMq(ctx context.Context, key, value []byte) error {
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
func (c *chatService) flushLoop(ctx context.Context) {
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

func (c *chatService) flushToDB() {
	log.Println("将缓存更新至数据库")

	// 核心技巧：将当前 buffer 赋值给临时变量，并创建一个新的空 map 给业务继续用
	// 这样锁占用的时间仅仅是 map 赋值的时间（纳秒级）
	c.bufferLock.Lock()
	pendingMap := c.buffer
	c.bufferLock.Unlock()

	// 遍历pending，依次进行更新
	for _, _ = range pendingMap {
		// 1. 将redis中seqID小于maxAckID的message设置为已删除

		// 2. 更新mysql中的conversation表，将last_ack_id更新为maxAckID
	}
}

// HandleRevokeMsg 处理撤回消息
func (c *chatService) HandleRevokeMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	// 1. 撤回发送给在线用户的消息；2. 撤回发送给离线用户的消息
	return nil
}

// HandleDeleteMsg 处理删除消息
func (c *chatService) HandleDeleteMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	return nil
}
