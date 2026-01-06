package repository

import (
	"GoChat/internel/model/dao"
	"GoChat/pkg/util"
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IGroupMsgRepo interface {
	IBaseRepository[dao.GroupMessageModel]
}

type GroupMsgRepo struct {
	db *gorm.DB
}

func NewGroupMsgRepo(db *gorm.DB) *GroupMsgRepo {
	return &GroupMsgRepo{
		db: db,
	}
}

func (r *GroupMsgRepo) getTx(ctx context.Context) *gorm.DB {
	if db := util.GetTx(ctx); db != nil {
		return db
	}
	return r.db
}

func (r *GroupMsgRepo) GetByID(ctx context.Context, id uint64) (*dao.GroupMessageModel, error) {
	return nil, nil
}
func (r *GroupMsgRepo) Create(ctx context.Context, entity *dao.GroupMessageModel) error {
	db := r.getTx(ctx).WithContext(ctx)
	if err := db.Model(&dao.GroupMessageModel{}).Create(entity).Error; err != nil {
		zap.L().Error("创建群消息失败", zap.Error(err))
		return err
	}
	return nil
}
func (r *GroupMsgRepo) Delete(ctx context.Context, entity *dao.GroupMessageModel) error {
	return nil
}
func (r *GroupMsgRepo) Update(ctx context.Context, entity *dao.GroupMessageModel) error {
	return nil
}
func (r *GroupMsgRepo) List(ctx context.Context, params QueryParams) ([]dao.GroupMessageModel, int64, error) {
	return nil, 0, nil
}
