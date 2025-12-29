package repository

import (
	"GoChat/internel/model/dao"
	"GoChat/pkg/util"
	"context"
	"errors"
	"log"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrCreate = errors.New("create Error")
)

type IChatRepo interface {
	IBaseRepository[dao.MessageModel]
	GetMsgsByLastSeqID(ctx context.Context, userID uint64, conversationID string, seqID uint64, limit int64) ([]dao.MessageModel, error)
	UpdateMsgStatus(ctx context.Context, conversationID string, seqID uint64, isPushed bool) error
}

type ChatRepo struct {
	db *gorm.DB
}

func NewChatRepo(db *gorm.DB) *ChatRepo {
	return &ChatRepo{
		db: db,
	}
}

func (r *ChatRepo) getTx(ctx context.Context) *gorm.DB {
	if tx := util.GetTx(ctx); tx != nil {
		return tx
	}
	return r.db
}
func (r *ChatRepo) GetByID(ctx context.Context, id uint64) (*dao.MessageModel, error) {
	return nil, nil
}
func (r *ChatRepo) Create(ctx context.Context, msg *dao.MessageModel) error {
	db := r.getTx(ctx)
	err := db.Model(&dao.MessageModel{}).WithContext(ctx).Create(msg).Error
	if err != nil {
		zap.L().Error("create message error", zap.Error(err))
		return ErrCreate
	}
	return nil
}
func (r *ChatRepo) Delete(ctx context.Context, msg *dao.MessageModel) error {
	return nil
}
func (r *ChatRepo) Update(ctx context.Context, msg *dao.MessageModel) error {
	return nil
}
func (r *ChatRepo) List(ctx context.Context, params QueryParams) ([]dao.MessageModel, int64, error) {
	db := r.getTx(ctx).WithContext(ctx)
	var msgs []dao.MessageModel
	var total int64

	base := db.Model(&dao.MessageModel{}).
		Where(params.Query, params.Args...)

	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	q := base
	if len(params.Selects) > 0 {
		q = q.Select(params.Selects)
	}
	if params.OrderBy != "" {
		q = q.Order(params.OrderBy)
	}
	if params.Limit > 0 {
		q = q.Limit(int(params.Limit))
	}
	if params.Offset > 0 {
		q = q.Offset(int(params.Offset))
	}
	if err := q.Find(&msgs).Error; err != nil {
		return nil, 0, err
	}
	return msgs, total, nil
}

func (r *ChatRepo) GetMsgsByLastSeqID(ctx context.Context, userID uint64, conversationID string, seqID uint64, limit int64) ([]dao.MessageModel, error) {
	var msgs []dao.MessageModel

	log.Printf("数据库查找: %s, %d", conversationID, seqID)
	err := r.getTx(ctx).
		Model(&dao.MessageModel{}).
		Where("conversation_id = ? AND seq_id > ?", conversationID, seqID).
		Order("seq_id ASC").
		Limit(int(limit)).
		Find(&msgs).Error

	return msgs, err
}

func (r *ChatRepo) UpdateMsgStatus(ctx context.Context, conversationID string, seqID uint64, isPushed bool) error {
	db := r.getTx(ctx)
	result := db.Model(&dao.MessageModel{}).WithContext(ctx).
		Where("conversation_id = ? AND seq_id = ?", conversationID, seqID).
		Update("is_pushed", isPushed)
	return result.Error
}
