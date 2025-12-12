package repository

import (
	"GoChat/internel/model/dao"
	"context"
	"errors"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrCreate = errors.New("create Error")
)

type IChatRepo interface {
	IBaseRepository[dao.MessageModel]
}

type chatRepo struct {
	db *gorm.DB
}

func NewChatRepo(db *gorm.DB) IChatRepo {
	return &chatRepo{
		db: db,
	}
}

func (r *chatRepo) getTx(ctx context.Context) *gorm.DB {
	return r.db
}

func (r *chatRepo) GetByID(ctx context.Context, id uint64) (*dao.MessageModel, error) {
	return nil, nil
}
func (r *chatRepo) Create(ctx context.Context, msg *dao.MessageModel) error {
	db := r.getTx(ctx)
	err := db.Model(&dao.MessageModel{}).WithContext(ctx).Create(msg).Error
	if err != nil {
		zap.L().Error("create message error", zap.Error(err))
		return ErrCreate
	}
	return nil
}
func (r *chatRepo) Delete(ctx context.Context, msg *dao.MessageModel) error {
	return nil
}
func (r *chatRepo) Update(ctx context.Context, msg *dao.MessageModel) error {
	return nil
}
func (r *chatRepo) List(ctx context.Context, params QueryParams) ([]dao.MessageModel, int64, error) {
	return nil, 0, nil
}
