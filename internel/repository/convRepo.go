package repository

import (
	"GoChat/internel/model/dao"
	"GoChat/internel/model/dto"
	"GoChat/pkg/util"
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IConvRepo interface {
	IBaseRepository[dao.Conversation]
	GetConvByUserID(ctx context.Context, userID uint64) ([]dao.Conversation, error)
	GetConvByConvID(ctx context.Context, conversationID string) (*dao.Conversation, error)
	GetConvByUserIDConvID(ctx context.Context, userID uint64, conversationIDs []string) ([]dao.Conversation, error)
	GetLastSeqID(ctx context.Context, userID uint64, conversationID string) (uint64, error)
	UpdateLastAck(ctx context.Context, userID uint64, conversationID string, lastAckID uint64) error
	BulkUpdateLastAck(ctx context.Context, updates []*dto.UpdatesAck) error
	UpdateSenderConversation(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, createdAt time.Time) error
	UpdateReceiverConversation(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, createdAt time.Time) error
}

type ConvRepo struct {
	db *gorm.DB
}

func NewConvRepo(db *gorm.DB) *ConvRepo {
	return &ConvRepo{
		db: db,
	}
}

func (r *ConvRepo) getTx(ctx context.Context) *gorm.DB {
	if tx := util.GetTx(ctx); tx != nil {
		return tx
	}
	return r.db
}
func (r *ConvRepo) GetByID(ctx context.Context, id uint64) (*dao.Conversation, error) {
	return nil, nil
}
func (r *ConvRepo) Create(ctx context.Context, entity *dao.Conversation) error {
	return nil
}
func (r *ConvRepo) Delete(ctx context.Context, entity *dao.Conversation) error {
	return nil
}
func (r *ConvRepo) Update(ctx context.Context, entity *dao.Conversation) error {
	return nil
}
func (r *ConvRepo) List(ctx context.Context, params QueryParams) ([]dao.Conversation, int64, error) {
	return nil, 0, nil
}

// GetConvByUserID 根据UserID获取该用户的所有会话
func (r *ConvRepo) GetConvByUserID(ctx context.Context, userID uint64) ([]dao.Conversation, error) {
	db := r.getTx(ctx)
	var conversations []dao.Conversation
	result := db.Model(&dao.Conversation{}).WithContext(ctx).
		Where("owner_id = ?", userID).
		Find(&conversations)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return conversations, nil
}

func (r *ConvRepo) GetConvByConvID(ctx context.Context, conversationID string) (*dao.Conversation, error) {
	db := r.getTx(ctx)
	var conversation dao.Conversation
	result := db.Model(&dao.Conversation{}).WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Find(&conversation)
	if result.Error != nil {
		return nil, result.Error
	}
	return nil, nil
}

// GetConvByUserIDConvID 根据UserID和ConvID批量获取会话
func (r *ConvRepo) GetConvByUserIDConvID(ctx context.Context, userID uint64, conversationIDs []string) ([]dao.Conversation, error) {
	db := r.getTx(ctx)
	var conversations []dao.Conversation
	result := db.Model(&dao.Conversation{}).WithContext(ctx).
		Where("user_id = ? and conversation_id in ?", userID, conversationIDs).
		Find(&conversations)

	if result.Error != nil {
		return nil, result.Error
	}
	return conversations, nil
}

func (r *ConvRepo) GetLastSeqID(ctx context.Context, userID uint64, conversationID string) (uint64, error) {
	db := r.getTx(ctx)
	var conversation dao.Conversation
	result := db.Model(&dao.Conversation{}).WithContext(ctx).
		Select("last_seq_id").
		Where("owner_id = ? and conversation_id = ?", userID, conversationID).Find(&conversation)
	if result.Error != nil {
		return 0, result.Error
	}
	return conversation.LastSeqID, nil
}

func (r *ConvRepo) UpdateLastAck(ctx context.Context, userID uint64, conversationID string, lastAckID uint64) error {
	db := r.getTx(ctx)
	result := db.Model(&dao.Conversation{}).WithContext(ctx).
		Where("owner_id = ? and conversation_id = ?", userID, conversationID).
		Updates(map[string]interface{}{
			"last_ack_id":  lastAckID,
			"unread_count": 0,
		})
	return result.Error
}

func (r *ConvRepo) BulkUpdateLastAck(ctx context.Context, updates []*dto.UpdatesAck) error {
	db := r.getTx(ctx)
	// 拼接sql

	var sql strings.Builder
	args := make([]interface{}, 0, 5*len(updates))
	sql.WriteString("UPDATE conversation SET last_ack_id = CASE")
	for _, item := range updates {
		sql.WriteString("WHEN owner_id = ? AND conversation_id = ? THEN ?")
		args = append(args, item.UserID, item.ConversationID, item.LastAckID)
	}
	sql.WriteString("ELSE last_ack_id END")
	sql.WriteString(", unread_count = 0")
	sql.WriteString(" WHERE ")
	for j, item := range updates {
		if j > 0 {
			sql.WriteString(" OR ")
		}
		sql.WriteString("(owner_id = ? AND conversation_id = ?)")
		args = append(args, item.UserID, item.ConversationID)
	}
	if err := db.Model(&dao.Conversation{}).WithContext(ctx).Exec(sql.String(), args...).Error; err != nil {
		return err
	}

	return nil
}

// UpdateSenderConversation 更新发送者会话
func (r *ConvRepo) UpdateSenderConversation(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, updatedAt time.Time) error {
	db := r.getTx(ctx)
	return db.Model(&dao.Conversation{}).WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "owner_id"}, {Name: "conversation_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"updated_at":   updatedAt,
			"last_seq_id":  gorm.Expr("GREATEST(last_seq_id, ?)", seqID),
			"last_ack_id":  seqID,
			"unread_count": 0,
		}), // 插入冲突时则进行更新操作
	}).Create(&dao.Conversation{
		OwnerID:        senderID,
		ConversationID: convID,
		OtherUserID:    receiverID,
		LastSeqID:      seqID,
		LastAckID:      seqID,
		UnreadCount:    0,
		UpdatedAt:      updatedAt,
	}).Error
}

// UpdateReceiverConversation 更新接收者会话
func (r *ConvRepo) UpdateReceiverConversation(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, updatedAt time.Time) error {
	db := r.getTx(ctx)
	return db.Model(&dao.Conversation{}).WithContext(ctx).Clauses(
		clause.OnConflict{
			Columns: []clause.Column{{Name: "owner_id"}, {Name: "conversation_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"updated_at":   updatedAt,
				"last_seq_id":  gorm.Expr("GREATEST(last_seq_id , ?)", seqID),
				"unread_count": gorm.Expr("unread_count + ?", 1),
			}),
		}).Create(&dao.Conversation{
		OwnerID:        receiverID,
		ConversationID: convID,
		OtherUserID:    senderID,
		LastSeqID:      seqID,
		LastAckID:      0,
		UnreadCount:    1,
		UpdatedAt:      updatedAt,
	}).Error
}
