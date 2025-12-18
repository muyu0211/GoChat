package repository

import (
	"GoChat/internel/model/dao"
	"GoChat/pkg/util"
	"context"
	"errors"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"log"
)

var (
	ErrCreate = errors.New("create Error")
)

type IChatRepo interface {
	IBaseRepository[dao.MessageModel]
	GetByConversationIDAfterSeqID(ctx context.Context, conversationID string, seqID uint64, limit int64) ([]dao.MessageModel, int64, error)
	UpdateMsgStatus(ctx context.Context, conversationID string, seqID uint64, isPushed bool) error
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
	if tx := util.GetTx(ctx); tx != nil {
		return tx
	}
	log.Println("未获取到事务句柄")
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
	db := r.getTx(ctx)
	var msgs []dao.MessageModel
	err := db.Model(&dao.MessageModel{}).WithContext(ctx).
		Limit(int(params.Limit)).
		Offset(int(params.Offset)).
		Order(params.OrderBy).
		Select(params.Selects).
		Where(params.Filters).
		Find(&msgs).Error
	if err != nil {
		return nil, 0, err
	}
	return msgs, int64(len(msgs)), nil
}

func (r *chatRepo) GetByConversationIDAfterSeqID(ctx context.Context, conversationID string, seqID uint64, limit int64) ([]dao.MessageModel, int64, error) {
	params := QueryParams{
		Selects: []string{"id", "content", "conversation_id", "sender_id", "receiver_id", "msg_type", "msg_status", "seq_id", "created_at"},
		Filters: map[string]interface{}{
			"conversation_id": conversationID,
			"seq_id >":        seqID,
		},
		OrderBy: "seq_id ASC",
		Limit:   limit,
	}
	return r.List(ctx, params)
}

func (r *chatRepo) UpdateMsgStatus(ctx context.Context, conversationID string, seqID uint64, isPushed bool) error {
	db := r.getTx(ctx)
	result := db.Model(&dao.MessageModel{}).WithContext(ctx).
		Where("conversation_id = ? AND seq_id = ?", conversationID, seqID).
		Update("is_pushed", isPushed)
	return result.Error
}
