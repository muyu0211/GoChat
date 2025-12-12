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

// 接口的实现(只对外暴露interface)
type userRepo struct {
	db *gorm.DB
}

// NewUserRepo 构造函数，进行依赖注入
func NewUserRepo(db *gorm.DB) IUserRepo {
	return &userRepo{db: db}
}

// 尝试获取事务句柄
func (r *userRepo) getTx(ctx context.Context) *gorm.DB {
	if tx := util.GetTx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (r *userRepo) GetByID(ctx context.Context, id uint64) (*dao.UserBasicModel, error) {
	//db := r.getTx(ctx)
	return nil, nil
}

func (r *userRepo) Create(ctx context.Context, user *dao.UserBasicModel) error {
	db := r.getTx(ctx)
	return db.Create(user).Error
}

func (r *userRepo) Delete(ctx context.Context, user *dao.UserBasicModel) error {
	return nil
}

func (r *userRepo) Update(ctx context.Context, user *dao.UserBasicModel) error {
	return nil
}

func (r *userRepo) List(ctx context.Context, params QueryParams) ([]dao.UserBasicModel, int64, error) {
	return nil, 0, nil
}

func (r *userRepo) GetByPhone(ctx context.Context, phone string) (*dao.UserBasicModel, error) {
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
func (r *userRepo) GetByEmail(ctx context.Context, email string) (*dao.UserBasicModel, error) {
	var user dao.UserBasicModel
	err := r.getTx(ctx).WithContext(ctx).Model(&dao.UserBasicModel{}).Where("email = ?", email).Take(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		zap.L().Warn("failed to get user by email", zap.Error(err))
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}
