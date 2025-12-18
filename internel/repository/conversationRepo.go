package repository

import (
	"GoChat/internel/model/dao"
	"GoChat/pkg/util"
	"context"
	"gorm.io/gorm"
)

type IConversationRepo interface {
	IBaseRepository[dao.ConversationModel]
	GetByUserID(ctx context.Context, userID uint64) ([]dao.ConversationModel, error)
	GetByConversationID(ctx context.Context, conversationID string) (*dao.ConversationModel, error)
	GetByConversationIDs(ctx context.Context, conversationIDs []string) ([]dao.ConversationModel, error)
}

type conversationRepo struct {
	db *gorm.DB
}

func NewConversationRepo(db *gorm.DB) IConversationRepo {
	return &conversationRepo{
		db: db,
	}
}

func (r *conversationRepo) getTx(ctx context.Context) *gorm.DB {
	if tx := util.GetTx(ctx); tx != nil {
		return tx
	}
	return r.db
}
func (r *conversationRepo) GetByID(ctx context.Context, id uint64) (*dao.ConversationModel, error) {
	return nil, nil
}
func (r *conversationRepo) Create(ctx context.Context, entity *dao.ConversationModel) error {
	return nil
}
func (r *conversationRepo) Delete(ctx context.Context, entity *dao.ConversationModel) error {
	return nil
}
func (r *conversationRepo) Update(ctx context.Context, entity *dao.ConversationModel) error {
	return nil
}
func (r *conversationRepo) List(ctx context.Context, params QueryParams) ([]dao.ConversationModel, int64, error) {
	return nil, 0, nil
}

func (r *conversationRepo) GetByUserID(ctx context.Context, userID uint64) ([]dao.ConversationModel, error) {
	db := r.getTx(ctx)
	var conversations []dao.ConversationModel
	result := db.Model(&dao.ConversationModel{}).WithContext(ctx).
		Where("owner_id = ?", userID).
		Find(&conversations)

	if result.Error != nil {
		return nil, result.Error
	}
	return conversations, nil
}

func (r *conversationRepo) GetByConversationID(ctx context.Context, conversationID string) (*dao.ConversationModel, error) {
	db := r.getTx(ctx)
	var conversation dao.ConversationModel
	result := db.Model(&dao.ConversationModel{}).WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Find(&conversation)
	if result.Error != nil {
		return nil, result.Error
	}
	return nil, nil
}

// GetByConversationIDs 批量获取会话
func (r *conversationRepo) GetByConversationIDs(ctx context.Context, conversationIDs []string) ([]dao.ConversationModel, error) {
	db := r.getTx(ctx)
	var conversations []dao.ConversationModel
	result := db.Model(&dao.ConversationModel{}).WithContext(ctx).
		Where("conversation_id in ?", conversationIDs).
		Find(&conversations)

	if result.Error != nil {
		return nil, result.Error
	}
	return conversations, nil
}
