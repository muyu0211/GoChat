package repository

import (
	"GoChat/internel/model/dao"
	"GoChat/pkg/util"
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrNotFound = errors.New("record not found")
)

// IUserRepo 用户仓库接口：具体实体接口（组合基础接口 + 扩展方法）
type IUserRepo interface {
	IBaseRepository[dao.UserBasicModel]
	GetByPhone(ctx context.Context, phone string) (*dao.UserBasicModel, error)
	GetByEmail(ctx context.Context, email string) (*dao.UserBasicModel, error)
}

type UserRepo struct {
	db *gorm.DB
}

// NewUserRepo 构造函数，进行依赖注入
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

// 尝试获取事务句柄
func (r *UserRepo) getTx(ctx context.Context) *gorm.DB {
	if tx := util.GetTx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (r *UserRepo) GetByID(ctx context.Context, id uint64) (*dao.UserBasicModel, error) {
	var user dao.UserBasicModel
	err := r.getTx(ctx).WithContext(ctx).Model(&dao.UserBasicModel{}).Where("id = ?", id).Take(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		zap.L().Warn("failed to get user by id", zap.Error(err))
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return &user, nil
}

func (r *UserRepo) Create(ctx context.Context, user *dao.UserBasicModel) error {
	db := r.getTx(ctx)
	err := db.Create(user).Error
	if isDuplicateError(err) {
		return gorm.ErrDuplicatedKey
	}
	return nil
}

func (r *UserRepo) Delete(ctx context.Context, user *dao.UserBasicModel) error {
	return nil
}

func (r *UserRepo) Update(ctx context.Context, user *dao.UserBasicModel) error {
	return nil
}

func (r *UserRepo) List(ctx context.Context, params QueryParams) ([]dao.UserBasicModel, int64, error) {
	return nil, 0, nil
}

func (r *UserRepo) GetByPhone(ctx context.Context, phone string) (*dao.UserBasicModel, error) {
	var user dao.UserBasicModel
	err := r.getTx(ctx).WithContext(ctx).Model(&dao.UserBasicModel{}).Where("phone = ?", phone).Take(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by phone: %w", err)
	}
	return &user, nil
}
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*dao.UserBasicModel, error) {
	var user dao.UserBasicModel
	db := r.getTx(ctx).WithContext(ctx)
	err := db.Model(&dao.UserBasicModel{}).Where("email = ?", email).Take(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		zap.L().Warn("failed to get user by email", zap.Error(err))
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}
