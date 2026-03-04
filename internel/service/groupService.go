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
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"

	"go.uber.org/zap"
)

type IGroupService interface {
	NewGroup(ctx context.Context, group *dto.CreateGroupReq) (*dto.CreateGroupResp, error)
	AddGroupMember(ctx context.Context, g *dto.GroupMembers) error
	StopPool()
}

type GroupService struct {
	redisCache   *cache.RedisCache
	groupRepo    *repository.GroupRepo
	groupMsgRepo *repository.GroupMsgRepo
	convRepo     *repository.ConvRepo

	seqFactory  *SeqFactoryService
	pushService *PushService
	tx          *TxManager

	// 存放群 id 的缓存池
	groupIdPool chan uint64
	poolLock    sync.RWMutex

	// 协程池
	pushPool *ants.Pool

	// kafka
	groupMsgBuffer map[uint64]*groupBatch // 后续可以考虑分片锁：降低锁粒度，将不同的群组取模分配到不同的buffer中
	bufferLock     sync.RWMutex
	ticker         *time.Ticker
	groupProducer  *mq.GroupMsgProducer
	groupConsumer  *mq.GroupMsgConsumer

	// 监控指标
	msgReceived   uint64
	msgPushed     uint64
	msgPushFailed uint64
	msgSaved      uint64
	msgSaveFailed uint64
	flushCount    uint64
	metricsLock   sync.Mutex
}

type groupBatch struct {
	memberIDs  []uint64
	msgs       []ws.Msg
	groupKeyID int64
}

func NewGroupService(rc *cache.RedisCache, cr *repository.ConvRepo, gp *repository.GroupRepo, gmr *repository.GroupMsgRepo,
	sf *SeqFactoryService, ps *PushService, tx *TxManager,
	producer *mq.GroupMsgProducer, consumer *mq.GroupMsgConsumer) *GroupService {

	pool, _ := ants.NewPool(500, ants.WithExpiryDuration(30*time.Second))
	gs := &GroupService{
		convRepo:      cr,
		redisCache:    rc,
		groupRepo:     gp,
		groupMsgRepo:  gmr,
		seqFactory:    sf,
		pushService:   ps,
		tx:            tx,
		pushPool:      pool,
		groupProducer: producer,
		groupConsumer: consumer,
		groupIdPool:   make(chan uint64, 1000),
	}

	// 注册消费者处理方法
	if consumer != nil {
		consumer.RegisterHandler(gs.handlerGroupMsgFromMQ)
		gs.groupMsgBuffer = make(map[uint64]*groupBatch)
		gs.ticker = time.NewTicker(200 * time.Millisecond)
	}

	util.SafeGo(func() {
		gs.run(context.Background()) // TODO: 此处的ctx未与服务端的生命周期绑定，服务关闭/重启时可能无法执行ctx.Done()逻辑
	})
	return gs
}

func (gs *GroupService) run(ctx context.Context) {
	if gs.groupConsumer == nil {
		panic("group msg consumer is nil")
	}
	// 1. 启动全局唯一的定时处理任务，定时处理从kafka中获取到的数据
	util.SafeGo(func() {
		gs.flushGroupMsg(ctx)
	})

	// 2. 启动group msg 消费者监听
	gs.groupConsumer.Consume(ctx)
}

// NewGroup 创建群组
func (gs *GroupService) NewGroup(ctx context.Context, g *dto.CreateGroupReq) (*dto.CreateGroupResp, error) {
	// 1. 生成群组id
	gID, err := gs.genGroupID(ctx)
	if err != nil {
		zap.L().Error("Failed to generate group id", zap.Error(err))
		return nil, err
	}

	// 2. 雪花算法生成主键id
	pID := util.GenSnowflakeID()

	// 3.1 对群成员去重
	memMap := make(map[uint64]byte)
	for _, mem := range g.Members {
		memMap[mem] = util.GroupManager
	}
	memMap[g.OwnerID] = util.GroupOwner

	// 3.2 群成员数量判断
	if int32(len(memMap)) > util.GroupDefaultMemNum {
		return nil, errors.New("群成员数量超出")
	}

	/**
	TODO: 优化方向：先往redis中缓存要创建的群聊：key=主键id-群id, value=set(群成员id)
		  开启异步线程进行数据库的创建工作
	*/

	// 开启事务
	err = gs.tx.ExecTx(ctx, func(txCtx context.Context) error {
		// 3. 创建群组
		if err = gs.groupRepo.Create(txCtx, &dao.GroupModel{
			ID:         pID,
			GroupID:    gID,
			Name:       g.Name,
			Avatar:     g.Avatar,
			OwnerID:    g.OwnerID, // 群主 ID
			Type:       1,
			MaxMembers: util.GroupDefaultMemNum,
		}); err != nil {
			return err
		}

		// 4. 创建群成员（group_member中批量插入)
		members := make([]dao.GroupMemberModel, 0, len(memMap))
		for memID, role := range memMap {
			members = append(members, dao.GroupMemberModel{
				GroupKeyID: pID,
				GroupID:    gID,
				UserID:     memID,
				Role:       role,
			})
		}
		if err = gs.groupRepo.CreateMemberBatch(txCtx, members, 100); err != nil {
			return err
		}

		// 5. 为所有群成员创建会话conversation（conversation表中批量插入） TODO：优化——>只给群主先创建conversation记录，其他群成员等待用户主动点击或者群内有消息再创建
		convs := make([]dao.ConversationModel, 0, len(memMap))
		for memID, _ := range memMap {
			convs = append(convs, dao.ConversationModel{
				ConversationID: strconv.FormatUint(gID, 10), // 使用群组ID作为会话 ID
				OwnerID:        memID,
			})
		}
		if err = gs.convRepo.CreateConvBatch(txCtx, convs, 100); err != nil {
			return err
		}

		return nil
	})

	// 事务执行出错，进行回滚后返回
	if err != nil {
		zap.L().Warn("Failed to create group", zap.Error(err))
		return nil, err
	}

	// 5. 刚创建的群作为热数据放入redis中 (key=group_id, value=Hash(群成员id-是否被禁言)) TODO：优化-> 考虑使用ZSet，score作为是否被禁言的标志（可以实现范围查询）
	groupIDKey := util.GetRedisGroupIDKey(gID)
	memRedisPayload := make(map[string]interface{}, len(memMap))
	for memID, _ := range memMap {
		memRedisPayload[strconv.FormatUint(memID, 10)] = "0" // 群刚被创建时，没有人被禁言
	}
	if err := gs.redisCache.HSet(ctx, groupIDKey, util.RedisGroupIDExpire, memRedisPayload); err != nil {
		zap.L().Warn("Failed to add group member to redis", zap.Error(err))
	}

	// 6. 将创建群聊成功的消息放入MQ
	gs.pushGroupCreateEventToMQ(gID, memMap, g.OwnerID)

	// 返回
	return &dto.CreateGroupResp{GroupKeyID: pID, GroupID: gID}, nil
}

// AddGroupMember 添加群成员
func (gs *GroupService) AddGroupMember(ctx context.Context, g *dto.GroupMembers) error {
	return nil
}

// pushGroupCreateEventToMQ 推送群聊创建成功消息
func (gs *GroupService) pushGroupCreateEventToMQ(groupID uint64, members map[uint64]byte, OwnerID uint64) {
	// 1. 提取 MemberIDs
	memberIDs := make([]uint64, 0, len(members))
	for mID := range members {
		memberIDs = append(memberIDs, mID)
	}

	// 2. 组装event信息
	event := &mq.GroupMsgEvent{
		Cmd:       ws.CmdSystem,
		GroupID:   groupID,
		SenderID:  OwnerID,
		MemberIDs: memberIDs,
		Content:   "群创建成功",
		TimeStamp: 0,
	}
	eventBytes, err := event.Serialize()
	if err != nil {
		zap.L().Error("event 序列化失败", zap.Error(err))
		return
	}

	// 3. 投递到MQ: 同一群组的消息发布到同一个分区中，保证消息有序
	pubCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := fmt.Sprintf("%s:%d", "group", groupID)
	if err = gs.groupProducer.Publish(pubCtx, []byte(key), eventBytes); err != nil {
		zap.L().Error("[Kafka] 发送消息失败", zap.Error(err))
	}
}

// handlerGroupMsgFromMQ 处理从MQ中获取到的群消息
func (gs *GroupService) handlerGroupMsgFromMQ(ctx context.Context, key, value []byte) error {
	log.Println("接收到群聊消息, key: ", string(key))
	// 1. 接收到群聊消息
	var event = &mq.GroupMsgEvent{}
	if err := event.Deserialize(value); err != nil {
		zap.L().Error("[Kafka] 反序列化失败", zap.Error(err))
		return nil
	}

	// 2. 验证消息完整性
	if event.GroupID == 0 || event.SenderID == 0 {
		zap.L().Error("[Kafka] 消息不完整: groupID or senderID is 0",
			zap.Uint64("groupID", event.GroupID),
			zap.Uint64("senderID", event.SenderID))
		return nil
	}

	// 3. 获取群成员列表
	memberIDs := event.MemberIDs
	if len(memberIDs) == 0 {
		// 从Redis获取最新成员列表
		if ids, err := gs.getGroupMembersFromRedis(ctx, event.GroupID); err == nil && len(ids) > 0 {
			memberIDs = ids
		}
	}

	// 4. 放入缓冲区用于聚合（需要加锁）
	gs.bufferLock.Lock()
	defer gs.bufferLock.Unlock()

	gID := event.GroupID
	batchs, ok := gs.groupMsgBuffer[gID]
	if !ok {
		batchs = &groupBatch{
			memberIDs:  memberIDs,
			msgs:       make([]ws.Msg, 0, 10),
			groupKeyID: event.GroupKeyID,
		}
		gs.groupMsgBuffer[gID] = batchs
	} else {
		// 问题：上一条消息发送时群里有200人，后来一人退出，群只有199人，此时将两条消息接收者不同的消息聚合则会造成消息丢失
		// 取并集更新群成员列表，已退出群的用户由客户端对消息进行处理
		if len(memberIDs) > 0 {
			batchs.memberIDs = append(batchs.memberIDs, memberIDs...)
			batchs.memberIDs = util.Uniq(batchs.memberIDs)
			batchs.groupKeyID = event.GroupKeyID
		}
	}

	// 5. 构造ReplyMsg并添加到缓冲区
	batchs.msgs = append(batchs.msgs, &ws.ReplyMsg{
		Cmd:            event.Cmd,
		ConversationID: strconv.FormatUint(event.GroupID, 10),
		Flag:           0,
		Content:        event.Content,
		SenderID:       event.SenderID,
		TimeStamp:      event.TimeStamp,
		SeqID:          event.SeqID,
	})

	// 更新监控指标
	gs.metricsLock.Lock()
	gs.msgReceived++
	gs.metricsLock.Unlock()
	return nil
}

// flushGroupMsg 定时调用处理缓冲区
func (gs *GroupService) flushGroupMsg(ctx context.Context) {
	defer gs.ticker.Stop()
	for {
		select {
		case <-ctx.Done(): // 服务关闭时进行最后一次处理
			gs.flushAll()
		case <-gs.ticker.C:
			gs.flushAll()
		}
	}
}

// flushAll 处理缓冲区中群消息: 1. 将消息推送给接收方; 2. 将消息落库
func (gs *GroupService) flushAll() {
	// 申请锁获取缓冲区数据后重置缓冲区
	gs.bufferLock.Lock()
	if len(gs.groupMsgBuffer) == 0 {
		gs.bufferLock.Unlock()
		return
	}
	snapshot := gs.groupMsgBuffer
	gs.groupMsgBuffer = make(map[uint64]*groupBatch)
	gs.bufferLock.Unlock()

	// 更新监控指标
	gs.metricsLock.Lock()
	gs.flushCount++
	gs.metricsLock.Unlock()

	// 使用协程池并发处理数据
	for gID, batch := range snapshot {
		// 注意：在循环中提交协程池任务，必须处理变量捕获问题(闭包变量问题)
		currGID := gID
		currBatch := batch

		// 1. 推送消息
		err1 := gs.pushPool.Submit(func() {
			pushCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			gs.pushService.BatchPush(pushCtx, currBatch.msgs, currBatch.memberIDs)
			cancel()
		})

		if err1 != nil {
			zap.L().Error("群消息批量推送提交失败", zap.Error(err1))
		}

		// 2. 将消息入库保存
		err2 := gs.pushPool.Submit(func() {
			gs.flushToDB(currGID, currBatch)

			// 更新监控指标
			gs.metricsLock.Lock()
			gs.msgSaved++
			gs.metricsLock.Unlock()
		})

		if err2 != nil {
			zap.L().Error("群消息批量入库提交失败", zap.Error(err2))
		}
	}
}

// flushToDB 将一个群组的消息进行刷入数据库
func (gs *GroupService) flushToDB(groupID uint64, batch *groupBatch) {
	// 对消息进行筛选
	currMsgs := make([]ws.Msg, 0, len(batch.msgs))
	for _, msg := range batch.msgs {
		m, _ := msg.(*ws.ReplyMsg)
		if m.Cmd == ws.CmdSystem {
			continue
		}
		currMsgs = append(currMsgs, msg)
	}
	if len(currMsgs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 带重试的写入
	if err := util.Retry(util.RetryMaxTimes, util.RetryInterval, func() error {
		e := gs.tx.ExecTx(ctx, func(txCtx context.Context) error {
			// 1. 批量创建 GroupMessage 记录
			groupMessages := make([]dao.GroupMessageModel, 0, len(currMsgs))
			var lastSeqID uint64
			var lastSenderID uint64

			for _, msg := range currMsgs {
				m, ok := msg.(*ws.ReplyMsg)
				if !ok {
					continue
				}

				groupMessages = append(groupMessages, dao.GroupMessageModel{
					GroupKeyID: batch.groupKeyID,
					GroupID:    groupID,
					SeqID:      m.SeqID,
					SenderID:   m.SenderID,
					Content:    m.Content,
					Type:       m.MsgType, // 默认为文本消息
					CreatedAt:  time.UnixMilli(m.TimeStamp),
				})

				// 记录最后一条消息的信息，用于更新conversation表
				lastSeqID = m.SeqID
				lastSenderID = m.SenderID
			}

			// 批量创建群消息
			if err := gs.groupMsgRepo.CreateBatch(txCtx, groupMessages, 100); err != nil {
				zap.L().Error("批量创建群消息失败", zap.Error(err))
				return err
			}

			// 2. 批量更新群成员的 conversation 表
			if len(batch.memberIDs) > 0 && lastSeqID > 0 {
				if err := gs.convRepo.UpdateGroupConversations(txCtx, batch.groupKeyID, batch.memberIDs, lastSeqID, lastSenderID); err != nil {
					zap.L().Error("批量更新群成员会话表失败", zap.Error(err))
					return err
				}
			}

			return nil
		})
		return e
	}); err != nil {
		zap.L().Error("群消息刷库失败: ", zap.Error(err))
	}
}

// getGroupMembersFromRedis 从Redis获取群成员列表
func (gs *GroupService) getGroupMembersFromRedis(ctx context.Context, groupID uint64) ([]uint64, error) {
	groupIDKey := util.GetRedisGroupIDKey(groupID)
	memberIDsMap, err := gs.redisCache.HGetAll(ctx, groupIDKey)
	if err != nil {
		zap.L().Error("从Redis获取群成员列表失败", zap.Error(err))
		return nil, err
	}

	memberIDs := make([]uint64, 0, len(memberIDsMap))
	for memberIDStr := range memberIDsMap {
		memberID, err := strconv.ParseUint(memberIDStr, 10, 64)
		if err != nil {
			zap.L().Error("解析群成员ID失败", zap.Error(err))
			continue
		}
		memberIDs = append(memberIDs, memberID)
	}

	return memberIDs, nil
}

// genGroupID 生成群聊ID
func (gs *GroupService) genGroupID(ctx context.Context) (uint64, error) {
	// 结合 redis 生成群号（id）
	select {
	case id := <-gs.groupIdPool:
		return id, nil
	default: // 群号用完了时继续取
		if err := gs.fillGroupIDPool(ctx); err != nil {
			return 0, err
		}

		// 填充完再取一次
		select {
		case id := <-gs.groupIdPool:
			return id, nil
		case <-time.After(500 * time.Millisecond):
			return 0, errors.New("timeout getting group id")
		}
	}
}

// fillGroupIDPool 当缓存池中的群id号段用完时，从redis中获取新的群id号段
func (gs *GroupService) fillGroupIDPool(ctx context.Context) error {
	if len(gs.groupIdPool) > 0 {
		return nil
	}
	// 0. 加锁防止并发问题
	gs.poolLock.Lock()
	defer gs.poolLock.Unlock()

	// 1. 双重检查锁的第二层：加锁后再次检查
	// 防止 A 和 B 同时通过了第一层检查，A 进货完了，B 拿到锁又进了一次货
	if len(gs.groupIdPool) > 0 {
		return nil
	}

	// 2. 从redis 中获取群id号段
	groupIDKey := util.GetRedisGroupIDOffsetKey()
	var step int64 = 800 // 一次最多取 900 个号
	// 3. 取出群id号段
	groupMaxID, err := gs.redisCache.IncrBy(ctx, groupIDKey, step)
	if err != nil {
		return err
	}

	startID := groupMaxID - step + 1

	// 4. 填充缓存池
	for i := startID; i <= groupMaxID; i++ {
		finalID := uint64(i) + util.GroupIDOffSet
		// ⚠️ 注意：这里必须是非阻塞写入，或者保证 Channel 容量足够
		select {
		case gs.groupIdPool <- finalID:
		default:
			zap.L().Warn("GroupID pool full, discarding id", zap.Uint64("id", finalID))
		}
	}
	return nil
}

// GetMetrics 获取监控指标
func (gs *GroupService) GetMetrics() map[string]uint64 {
	gs.metricsLock.Lock()
	defer gs.metricsLock.Unlock()

	return map[string]uint64{
		"msgReceived":   gs.msgReceived,
		"msgPushed":     gs.msgPushed,
		"msgPushFailed": gs.msgPushFailed,
		"msgSaved":      gs.msgSaved,
		"msgSaveFailed": gs.msgSaveFailed,
		"flushCount":    gs.flushCount,
	}
}

// StopPool 停止协程池
func (gs *GroupService) StopPool() {
	if gs.pushPool != nil {
		gs.pushPool.Release()
	}
}
