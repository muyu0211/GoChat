package service

import (
	"GoChat/internel/model/dto"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	"context"
	"log"
)

type IGroupService interface {
	NewGroup(ctx context.Context, group *dto.CreateGroupReq) (*dto.CreateGroupResp, error)
}

type GroupService struct {
	convRepo   *repository.ConvRepo
	redisCache *cache.RedisCache
}

func NewGroupService(convRepo *repository.ConvRepo, redisCache *cache.RedisCache) *GroupService {
	return &GroupService{
		convRepo:   convRepo,
		redisCache: redisCache,
	}
}

func (gs *GroupService) NewGroup(ctx context.Context, group *dto.CreateGroupReq) (*dto.CreateGroupResp, error) {
	log.Print("创建群组")
	return nil, nil
}
