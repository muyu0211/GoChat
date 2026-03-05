package service

import (
	"GoChat/internel/infrastructure/mq"
	"GoChat/internel/model/dao"
	"GoChat/internel/model/dto"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	ws "GoChat/internel/websocket"
	"GoChat/pkg/util"
	cnst "GoChat/pkg/util/const"
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type IChatService interface {
	HandleSingleChatMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleGroupChatMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleAckMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleRevokeMsg(context.Context, *ws.Client, *ws.SendMsg) error
	HandleDeleteMsg(context.Context, *ws.Client, *ws.SendMsg) error
}

type ChatService struct {
	seqFactory       *SeqFactoryService
	pushService      *PushService
	userService      *UserService
	chatRepo         *repository.ChatRepo
	convRepo         *repository.ConvRepo
	groupChatRepo    *repository.GroupMsgRepo
	groupRepo        *repository.GroupRepo
	redisCache       *cache.RedisCache
	tx               *TxManager
	producer         *mq.AckProducer
	consumer         *mq.AckConsumer
	groupMsgProducer *mq.GroupMsgProducer

	// 内存缓冲区: Key: "ConversationID" (聚合维度)；Value: MaxSeqID (聚合结果)
	buffer        map[string]uint64
	bufferLock    sync.RWMutex
	batchSize     int
	flushInterval time.Duration
}

func NewChatService(sq *SeqFactoryService, ps *PushService, us *UserService, cr *repository.ChatRepo, ccr *repository.ConvRepo,
	gmr *repository.GroupMsgRepo, gr *repository.GroupRepo, tx *TxManager, rc *cache.RedisCache,
	producer *mq.AckProducer, consumer *mq.AckConsumer, grpMsgProd *mq.GroupMsgProducer) *ChatService {
	cs := &ChatService{
		seqFactory:       sq,
		pushService:      ps,
		userService:      us,
		chatRepo:         cr,
		convRepo:         ccr,
		groupChatRepo:    gmr,
		groupRepo:        gr,
		tx:               tx,
		redisCache:       rc,
		producer:         producer,
		consumer:         consumer,
		groupMsgProducer: grpMsgProd,
		buffer:           make(map[string]uint64),
		flushInterval:    5 * time.Second,
	}

	// 注册ack消息处理函数, 注意：handler 必须是并发安全的
	if consumer != nil {
		consumer.RegisterHandler(cs.handlerAckFromMq)

		// 启动ack消费者监听
		util.SafeGo(func() {
			cs.run(context.Background())
		})
	}
	return cs
}

// Run 启动kafka消费服务
func (c *ChatService) run(ctx context.Context) {
	if c.consumer == nil {
		panic("ack consumer is nil")
	}
	// 1. 启动定时刷库协程
	util.SafeGo(func() {
		c.flushLoop(ctx)
	})

	// 2. 启动消费者 (Consumer - 阻塞运行)
	c.consumer.Consume(ctx)
}

// HandleSingleChatMsg 处理单聊上行消息(用户发送过来的消息): 整个"上行"流程：参数校验 -> 接收方检查 -> 去重 -> 取号 -> 落库 -> (给客户端)返回 ACK 数据。
func (c *ChatService) HandleSingleChatMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	senderID := client.UserID
	req.SenderID = senderID
	createdAt := time.Now().UTC()
	ack := &ws.ReplyMsg{
		Cmd:         ws.CmdAck,
		ReceiverID:  senderID,
		AckID:       req.SeqID,
		ClientMsgID: req.ClientMsgID,
		TimeStamp:   createdAt.UnixMilli(),
	}

	// 1. 获取并校验会话ID
	if req.ConversationID != "" {
		converID := req.ConversationID
		// 校验 conversationID 格式：格式为"发送方ID_接收方ID"
		parts := strings.Split(converID, "_")
		if len(parts) != 2 {
			zap.L().Error("conversationID格式错误", zap.String("conversationID", converID))
			c.pushMsg(ctx, ack.SetContent("conversationID格式错误"))
			return errors.New("conversationID格式错误")
		}
		// 校验 conversationID 是否包含发送方和接收方
		id1, err1 := strconv.ParseUint(parts[0], 10, 64)
		id2, err2 := strconv.ParseUint(parts[1], 10, 64)
		if err1 != nil || err2 != nil {
			zap.L().Error("conversationID中的ID格式错误", zap.String("conversationID", converID))
			c.pushMsg(ctx, ack.SetContent("conversationID中的ID格式错误"))
			return errors.New("conversationID中的ID格式错误")
		}
		// 确保 conversationID 包含发送方和接收方（顺序可以不同）
		if (id1 != senderID && id1 != req.ReceiverID) || (id2 != senderID && id2 != req.ReceiverID) {
			zap.L().Error("conversationID与发送方/接收方不匹配",
				zap.String("conversationID", converID),
				zap.Uint64("senderID", senderID),
				zap.Uint64("receiverID", req.ReceiverID))
			c.pushMsg(ctx, ack.SetContent("conversationID与发送方/接收方不匹配"))
			return errors.New("conversationID与发送方/接收方不匹配")
		}
	} else {
		req.ConversationID = util.GetConversationID(senderID, req.ReceiverID)
	}

	// 2. 检查接收方是否存在且账户状态可用
	exists, err := c.userService.CheckUserExistsAndActive(ctx, req.ReceiverID)
	if err != nil {
		zap.L().Error("检查接收方状态失败", zap.Uint64("receiverID", req.ReceiverID), zap.Error(err))
		return err
	}
	if !exists {
		zap.L().Warn("接收方不存在或账户不可用", zap.Uint64("receiverID", req.ReceiverID))
		c.pushMsg(ctx, ack.SetContent("接收方不存在或账户不可用"))
		return errors.New("接收方不存在或账户不可用")
	}

	// 3. 幂等性检查 和 生成会话ID （redis事务实现）
	isDup, seqID, err := c.seqFactory.CheckAndSetDedupWithSeq(ctx, senderID, req.ConversationID, req.ClientMsgID, util.RedisDupExpire)
	if err != nil {
		zap.L().Warn("消息去重/取号失败", zap.Error(err), zap.String("conversationID", req.ConversationID))
	}
	if isDup { // 消息重复
		zap.L().Warn("消息重复", zap.String("conversationID", req.ConversationID))
		util.SafeGo(func() {
			c.pushMsg(ctx, ack.SetContent("消息重复"))
		})
		return nil
	}

	// 4. 消息落库（事务操作：message表新增记录 + conversation表更新相应字段）
	err = util.Retry(util.RetryMaxTimes, util.RetryInterval, func() error {
		exErr := c.tx.ExecTx(ctx, func(ctx context.Context) error {
			// 创建 message 表记录
			err = c.chatRepo.Create(ctx, &dao.MessageModel{
				ConversationID: req.ConversationID,
				SeqID:          seqID,
				SenderID:       req.SenderID,
				ReceiverID:     req.ReceiverID,
				Content:        req.Content,
				MsgType:        req.MsgType,
				MsgStatus:      util.MsgStatusRead,
				CreatedAt:      createdAt,
			})

			// 更新 conversation 表中发送方记录
			err = c.convRepo.UpdateSenderConversation(ctx, req.SenderID, req.ReceiverID, req.ConversationID, seqID, createdAt)

			// 更新 conversation 表中接收方记录
			err = c.convRepo.UpdateReceiverConversation(ctx, req.SenderID, req.ReceiverID, req.ConversationID, seqID, createdAt)

			if err != nil {
				return err
			}
			return nil
		})
		return exErr
	})

	if err != nil {
		zap.L().Error("消息落库失败", zap.String("conversationID", req.ConversationID), zap.Error(err))
		return err
	}

	// 5. 异步更新redis中双方会话表缓存(Set存储)
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

	// 6. 向客户端回复ACK
	util.SafeGo(func() {
		ack := &ws.ReplyMsg{
			Cmd:            ws.CmdAck,
			ConversationID: req.ConversationID,
			SenderID:       req.SenderID,
			ReceiverID:     req.SenderID,
			ClientMsgID:    req.ClientMsgID,
			AckID:          seqID,
			TimeStamp:      createdAt.UnixMilli(),
		}
		c.pushMsg(ctx, ack)
	})

	// 7. 异步调用下行推送服务（带重试+死信队列）
	util.SafeGo(func() {
		reply := &ws.ReplyMsg{
			Cmd:            ws.CmdSingleChat,
			ConversationID: req.ConversationID,
			ClientMsgID:    req.ClientMsgID,
			SeqID:          seqID,
			SenderID:       senderID,
			ReceiverID:     req.ReceiverID,
			Content:        req.Content,
			TimeStamp:      createdAt.UnixMilli(),
		}
		c.pushMsg(ctx, reply)
	})

	return nil
}

// HandleGroupChatMsg 处理群聊上行消息: 整个"上行"流程：参数校验 -> 发送方检查 -> 去重 -> 取号 -> 发送给MQ -> 落库 -> (给客户端)返回 ACK 数据。
func (c *ChatService) HandleGroupChatMsg(ctx context.Context, client *ws.Client, req *ws.SendMsg) error {
	var groupID uint64
	senderID := client.UserID
	createdAt := time.Now().UTC()
	ack := &ws.ReplyMsg{
		Cmd:         ws.CmdAck,
		SenderID:    senderID,
		ReceiverID:  senderID,
		AckID:       req.SeqID,
		ClientMsgID: req.ClientMsgID,
		TimeStamp:   createdAt.UnixMilli(),
	}

	// 0. 获取会话ID
	if req.ConversationID != "" {
		groupID, _ = strconv.ParseUint(req.ConversationID, 10, 64)
	} else {
		return errors.New("会话 ID 不能为空")
	}

	// 1. a.判断群聊是否存在
	groupIDKey := util.GetRedisGroupIDKey(groupID)
	if exist, err := c.redisCache.Exists(ctx, groupIDKey); err != nil {
		zap.L().Warn("判断用户是否在群聊中失败", zap.Error(err), zap.String("groupIDKet: ", groupIDKey))
		return err
	} else if !exist {
		// TODO: 查找数据库，判断群聊是否存在
		dbExist, err := c.groupChatRepo.FindGroupByID(ctx, groupID)
		if err != nil {
			zap.L().Error("查找群聊失败", zap.Error(err))
			return err
		}
		if !dbExist {
			c.pushMsg(ctx, ack.SetContent("群聊不存在"))
			return errors.New("群聊不存在")
		}
		// TODO：将写入缓存, Hash: key: group, field: userID, value: MsgStatusNormal
	}

	// 1. b.判断发送方是否在群聊中/是否被禁言
	status, exists, err := c.redisCache.HGetWithExists(ctx, groupIDKey, strconv.FormatUint(req.SenderID, 10))
	if err != nil {
		zap.L().Error("判断用户是否在群聊中失败", zap.Error(err), zap.String("status", status))
		return err
	}
	if !exists {
		dbExists, err := c.groupRepo.FindGroupUser(ctx, groupID, req.SenderID)
		if err != nil {
			zap.L().Error("判断用户是否在群聊中失败", zap.Error(err))
			return err
		}
		if !dbExists {
			c.pushMsg(ctx, ack.SetContent("发送方不在群聊中"))
			return errors.New("发送方不在群聊中")
		}
	} else if status != cnst.MsgStatusNormal {
		if status == cnst.MsgStatusMuted {
			c.pushMsg(ctx, ack.SetContent("发送方被禁言"))
			return errors.New("发送方被禁言")
		}
	}

	// 2. 幂等性检查、生成会话ID （lua脚本保证redis原子性）
	isDup, seqID, err := c.seqFactory.CheckAndSetDedupWithSeq(ctx, senderID, req.ConversationID, req.ClientMsgID, util.RedisDupExpire)
	if err != nil {
		zap.L().Error("消息去重/取号失败", zap.Error(err))
	}
	if isDup {
		c.pushMsg(ctx, ack.SetContent("消息重复"))
		return nil
	}

	// 3. 回复 ACK
	util.SafeGo(func() {
		ack := &ws.ReplyMsg{
			Cmd:            ws.CmdAck,
			ConversationID: fmt.Sprintf("%d", groupID),
			ReceiverID:     req.SenderID,
			ClientMsgID:    req.ClientMsgID,
			SeqID:          req.SeqID,
			TimeStamp:      createdAt.UnixMilli(),
		}
		c.pushMsg(ctx, ack)
	})

	// 4. 构造mq消息体
	groupMsg := &mq.GroupMsgEvent{
		Cmd:         ws.CmdGroupChat,
		GroupID:     groupID,
		SenderID:    senderID,
		Content:     req.Content,
		TimeStamp:   createdAt.UnixMilli(),
		ClientMsgID: req.ClientMsgID,
		SeqID:       seqID,
		MsgType:     req.MsgType,
	}

	// 5. 发送mq消息: 同一群组的消息发布到同一个分区中，保证消息有序
	msgBytes, err := groupMsg.Serialize()
	if err != nil {
		zap.L().Error("序列化群消息失败", zap.Error(err))
		return err
	}
	pubCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	util.SafeGo(func() {
		key := fmt.Sprintf("%s:%d", "group", groupID)
		err = c.groupMsgProducer.Publish(pubCtx, []byte(key), msgBytes)
		if err != nil {
			zap.L().Error("发送群消息到kafka失败", zap.Error(err))
			return
		}
	})

	// // 进行落库及后续逻辑处理：先写数据库，再写缓存，流程上尽量保证一致性
	// // 创建 message 表记录
	// msg := &dao.GroupMessageModel{
	// 	GroupKeyID: req.GroupKeyID,
	// 	GroupID:    groupID,
	// 	SeqID:      seqID,
	// 	SenderID:   senderID,
	// 	Content:    req.Content,
	// }
	// if err := c.groupChatRepo.Create(ctx, msg); err != nil {
	// 	zap.L().Error("消息落库失败", zap.Error(err))
	// 	return err
	// }

	return nil
}

// pushMsg 通用推送消息方法：将消息推送给客户端
func (c *ChatService) pushMsg(ctx context.Context, reply ws.Msg) {
	pushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.pushService.Push(pushCtx, []ws.Msg{reply}, reply.GetReceiverID()); err != nil {
		zap.L().Error("消息推送失败",
			zap.Uint64("sender_id", reply.GetSenderID()),
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

	if err := c.convRepo.BulkUpdateLastAck(ctx, batch); err != nil {
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
