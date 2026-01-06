package repository

import (
	"GoChat/internel/model/dao"
	"GoChat/pkg/util"
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IGroupRepo interface {
	IBaseRepository[dao.GroupModel]
	CreateMemberBatch(ctx context.Context, members []dao.GroupMemberModel, batchSize int) error
}

type GroupRepo struct {
	db *gorm.DB
}

func NewGroupRepo(db *gorm.DB) *GroupRepo {
	return &GroupRepo{
		db: db,
	}
}

func (r *GroupRepo) getTx(ctx context.Context) *gorm.DB {
	if db := util.GetTx(ctx); db != nil {
		return db
	}
	return r.db
}

func (r *GroupRepo) GetByID(ctx context.Context, id uint64) (*dao.GroupModel, error) {
	return nil, nil
}
func (r *GroupRepo) Create(ctx context.Context, entity *dao.GroupModel) error {
	db := r.getTx(ctx).WithContext(ctx)
	if err := db.Model(&dao.GroupModel{}).Create(entity).Error; err != nil {
		zap.L().Error("创建群失败", zap.Error(err))
		return err
	}
	return nil
}
func (r *GroupRepo) Delete(ctx context.Context, entity *dao.GroupModel) error {
	return nil
}
func (r *GroupRepo) Update(ctx context.Context, entity *dao.GroupModel) error {
	return nil
}
func (r *GroupRepo) List(ctx context.Context, params QueryParams) ([]dao.GroupModel, int64, error) {
	return nil, 0, nil
}

func (r *GroupRepo) CreateMemberBatch(ctx context.Context, members []dao.GroupMemberModel, batchSize int) error {
	db := r.getTx(ctx).WithContext(ctx)
	if err := db.Model(&dao.GroupMemberModel{}).CreateInBatches(members, batchSize).Error; err != nil {
		zap.L().Error("批量插入群成员失败", zap.Error(err))
		return err
	}
	return nil
}
