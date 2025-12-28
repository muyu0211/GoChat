package service

import (
	"GoChat/internel/model/dto"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	"context"
)

type IGroupService interface {
	NewGroup(ctx context.Context, group *dto.CreateGroupReq) (*dto.CreateGroupResp, error)
}

type GroupService struct {
	convRepo   repository.IConversationRepo
	redisCache cache.ICacheRepository
}

func NewGroupService(convRepo repository.IConversationRepo, redisCache cache.ICacheRepository) IGroupService {
	return &GroupService{
		convRepo:   convRepo,
		redisCache: redisCache,
	}
}

func (gs *GroupService) NewGroup(ctx context.Context, group *dto.CreateGroupReq) (*dto.CreateGroupResp, error) {
	return nil, nil
}
