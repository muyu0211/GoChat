package service

import (
	"GoChat/internel/model/dao"
	"GoChat/internel/model/dto"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	ws "GoChat/internel/websocket"
	"GoChat/pkg/util"
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

type IGroupService interface {
	NewGroup(ctx context.Context, group *dto.CreateGroupReq) (*dto.CreateGroupResp, error)
}

type GroupService struct {
	redisCache *cache.RedisCache
	groupRepo  *repository.GroupRepo
	convRepo   *repository.ConvRepo

	seqFactory  *SeqFactoryService
	pushService *PushService
	tx          *TxManager

	// 存放群 id 的缓存池
	groupIdPool chan uint64
	poolLock    sync.RWMutex
}

func NewGroupService(rc *cache.RedisCache, cr *repository.ConvRepo, gp *repository.GroupRepo, sf *SeqFactoryService, ps *PushService, tx *TxManager) *GroupService {
	gs := &GroupService{
		convRepo:    cr,
		redisCache:  rc,
		groupRepo:   gp,
		seqFactory:  sf,
		pushService: ps,
		tx:          tx,
	}
	gs.groupIdPool = make(chan uint64, 1000)
	return gs
}

func (gs *GroupService) NewGroup(ctx context.Context, g *dto.CreateGroupReq) (*dto.CreateGroupResp, error) {
	// 1. 生成群组id
	gID, err := gs.genGroupID(ctx)
	if err != nil {
		zap.L().Error("Failed to generate group id", zap.Error(err))
		return nil, err
	}

	// 2. 雪花算法生成主键id
	pID := util.GenSnowflakeID()

	// 3. 对群成员去重
	memMap := make(map[uint64]byte)
	for _, mem := range g.Members {
		if g.OwnerID == mem {
			memMap[mem] = 2
			continue
		}
		memMap[mem] = 1
	}

	/**
	TODO: 优化方向：先往redis中缓存要创建的群聊：key=主键id-群id, value=set(群成员id)
		  开启异步线程进行数据库的创建工作
	*/

	// 开启事务
	err = gs.tx.ExecTx(ctx, func(ctx context.Context) error {
		// 3. 创建群组
		if err := gs.groupRepo.Create(ctx, &dao.GroupModel{
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
		if err := gs.groupRepo.CreateMemberBatch(ctx, members, 100); err != nil {
			return err
		}

		// 5. 为所有群成员创建会话conversation（conversation表中批量插入）
		convs := make([]dao.ConversationModel, 0, len(memMap))
		for memID, _ := range memMap {
			convs = append(convs, dao.ConversationModel{
				ConversationID: strconv.FormatInt(pID, 10), // 使用群组的主键 ID作为会话 ID
				OwnerID:        memID,
			})
		}
		if err := gs.convRepo.CreateConvBatch(ctx, convs, 100); err != nil {
			return err
		}

		return nil
	})

	// 事务执行出错，进行回滚后返回
	if err != nil {
		zap.L().Error("Failed to create group", zap.Error(err))
		return nil, err
	}

	// 5. 刚创建的群作为热数据放入redis中 (key=group_id, value=Hash(群成员id-是否被禁言))
	groupIDKey := util.GetRedisGroupIDKey(gID)
	memRedisPayload := make(map[string]interface{}, len(memMap))
	for memID, _ := range memMap {
		memRedisPayload[strconv.FormatUint(memID, 10)] = "0" // 群刚被创建时，没有人被禁言
	}
	if err := gs.redisCache.HSet(ctx, groupIDKey, util.RedisGroupIDExpire, memRedisPayload); err != nil {
		zap.L().Error("Failed to add group member to redis", zap.Error(err))
	}

	// 6. 异步将创建群聊成功的消息push给每个群成员（Push 接口）
	gs.pushGroupCreateEvent(gID, memMap)
	return &dto.CreateGroupResp{GroupKeyID: pID, GroupID: gID}, nil
}

func (gs *GroupService) pushGroupCreateEvent(groupID uint64, members map[uint64]byte) {
	pushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	var wg sync.WaitGroup

	for memberID, _ := range members {
		wg.Add(1)
		util.SafeGoWithArgs(func(args ...interface{}) {
			defer wg.Done()
			memberID = args[0].(uint64)
			pushPayLoad := &ws.ReplyMsg{
				Cmd:        ws.CmdNotice,
				Content:    "群创建成功",
				ReceiverID: memberID,
			}
			_ = gs.pushService.Push(pushCtx, pushPayLoad)
		}, memberID)
	}
	util.SafeGo(func() {
		wg.Wait()
		cancel()
	})
}

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
