package repository

import (
	"GoChat/internel/model/dao"
	"GoChat/pkg/util"
	"context"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IGroupMsgRepo interface {
	IBaseRepository[dao.GroupMessageModel]
	CreateBatch(ctx context.Context, entities []dao.GroupMessageModel, batchSize int) error
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

// isDuplicateKeyError 判断是否是 MySQL 的唯一键冲突错误 (Error 1062)
func isDuplicateKeyErrorGroupMsg(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return true
	}
	return strings.Contains(err.Error(), "1062") || strings.Contains(err.Error(), "Duplicate entry")
}

// CreateBatch 批量创建群消息（幂等性保证：忽略唯一键冲突）
func (r *GroupMsgRepo) CreateBatch(ctx context.Context, entities []dao.GroupMessageModel, batchSize int) error {
	db := r.getTx(ctx).WithContext(ctx)

	// 尝试批量插入
	err := db.CreateInBatches(entities, batchSize).Error

	// 如果是唯一键冲突错误，说明消息已存在，直接返回成功
	if err != nil && isDuplicateKeyErrorGroupMsg(err) {
		zap.L().Info("批量创建群消息：检测到重复消息，忽略", zap.Int("count", len(entities)))
		return nil
	}

	if err != nil {
		zap.L().Error("批量创建群消息失败", zap.Error(err))
		return err
	}
	return nil
}
