package repository

import (
	"GoChat/internel/model/dao"
	"GoChat/pkg/util"
	"context"

	"gorm.io/gorm"
)

type IGroupRepo interface {
	IBaseRepository[dao.GroupModel]
}

type GroupRepo struct {
	db *gorm.DB
}

func NewGroupRepo(db *gorm.DB) *GroupRepo {
	return &GroupRepo{
		db: db,
	}
}

func (gr *GroupRepo) getTx(ctx context.Context) *gorm.DB {
	if db := util.GetTx(ctx); db != nil {
		return db
	}
	return gr.db
}

func (gr *GroupRepo) GetByID(ctx context.Context, id uint64) (*dao.GroupModel, error) {
	return nil, nil
}
func (gr *GroupRepo) Create(ctx context.Context, entity *dao.GroupModel) error {
	return nil
}
func (gr *GroupRepo) Delete(ctx context.Context, entity *dao.GroupModel) error {
	return nil
}
func (gr *GroupRepo) Update(ctx context.Context, entity *dao.GroupModel) error {
	return nil
}
func (gr *GroupRepo) List(ctx context.Context, params QueryParams) ([]dao.GroupModel, int64, error) {
	return nil, 0, nil
}
