package service

import (
	"GoChat/internel/model/dao"
	"GoChat/internel/model/dto"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	"GoChat/pkg/util"
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

type IGroupService interface {
	NewGroup(ctx context.Context, group *dto.CreateGroupReq) (*dto.CreateGroupResp, error)
}

type GroupService struct {
	seqFactory *SeqFactoryService
	redisCache *cache.RedisCache
	groupRepo  *repository.GroupRepo
	convRepo   *repository.ConvRepo

	// 存放群 id 的缓存池
	groupIdPool chan uint64
	poolLock    sync.RWMutex
}

func NewGroupService(rc *cache.RedisCache, sf *SeqFactoryService, cr *repository.ConvRepo, gp *repository.GroupRepo) *GroupService {
	gs := &GroupService{
		convRepo:   cr,
		redisCache: rc,
		seqFactory: sf,
		groupRepo:  gp,
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

	// 3. 创建群组
	group := &dao.GroupModel{
		ID:           pID,
		GroupID:      gID,
		Name:         g.Name,
		Avatar:       g.Avatar,
		OwnerID:      g.OwnerID,
		Type:         0,
		MaxMembers:   util.GroupDefaultMemNum,
		IsMuteAll:    false,
		JoinType:     0,
		Notification: "",
		Status:       0,
		CreatedAt:    time.Time{},
		UpdatedAt:    time.Time{},
	}
	if err := gs.groupRepo.Create(ctx, group); err != nil {
		zap.L().Error("Failed to create group", zap.Error(err))
		return nil, err
	}

	// 4. 创建群组会话

	// 5. 刚创建的群作为热数据放入redis中

	return nil, nil
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
	groupIDKey := util.GetRedisGroupIdKey()
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
