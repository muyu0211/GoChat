package repository

import (
	"GoChat/internel/model/dao"
	"GoChat/internel/model/dto"
	"GoChat/pkg/util"
	"context"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IConversationRepo interface {
	IBaseRepository[dao.Conversation]
	GetByUserID(ctx context.Context, userID uint64) ([]dao.Conversation, error)
	GetByConversationID(ctx context.Context, conversationID string) (*dao.Conversation, error)
	GetByConversationIDs(ctx context.Context, conversationIDs []string) ([]dao.Conversation, error)
	UpdateLastAck(ctx context.Context, userID uint64, conversationID string, lastAckID uint64) error
	BulkUpdateLastAck(ctx context.Context, updates []*dto.UpdatesAck) error
	UpdateSenderConversation(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, createdAt time.Time) error
	UpdateReceiverConversation(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, createdAt time.Time) error
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
		log.Println("获取到事务句柄")
		return tx
	}
	return r.db
}
func (r *conversationRepo) GetByID(ctx context.Context, id uint64) (*dao.Conversation, error) {
	return nil, nil
}
func (r *conversationRepo) Create(ctx context.Context, entity *dao.Conversation) error {
	return nil
}
func (r *conversationRepo) Delete(ctx context.Context, entity *dao.Conversation) error {
	return nil
}
func (r *conversationRepo) Update(ctx context.Context, entity *dao.Conversation) error {
	return nil
}
func (r *conversationRepo) List(ctx context.Context, params QueryParams) ([]dao.Conversation, int64, error) {
	return nil, 0, nil
}

func (r *conversationRepo) GetByUserID(ctx context.Context, userID uint64) ([]dao.Conversation, error) {
	db := r.getTx(ctx)
	var conversations []dao.Conversation
	result := db.Model(&dao.Conversation{}).WithContext(ctx).
		Where("owner_id = ?", userID).
		Find(&conversations)

	if result.Error != nil {
		return nil, result.Error
	}
	return conversations, nil
}

func (r *conversationRepo) GetByConversationID(ctx context.Context, conversationID string) (*dao.Conversation, error) {
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

// GetByConversationIDs 批量获取会话
func (r *conversationRepo) GetByConversationIDs(ctx context.Context, conversationIDs []string) ([]dao.Conversation, error) {
	db := r.getTx(ctx)
	var conversations []dao.Conversation
	result := db.Model(&dao.Conversation{}).WithContext(ctx).
		Where("conversation_id in ?", conversationIDs).
		Find(&conversations)

	if result.Error != nil {
		return nil, result.Error
	}
	return conversations, nil
}

func (r *conversationRepo) UpdateLastAck(ctx context.Context, userID uint64, conversationID string, lastAckID uint64) error {
	db := r.getTx(ctx)
	result := db.Model(&dao.Conversation{}).WithContext(ctx).
		Where("owner_id = ? and conversation_id = ?", userID, conversationID).
		Updates(map[string]interface{}{
			"last_ack_id":  lastAckID,
			"unread_count": 0,
		})
	return result.Error
}

func (r *conversationRepo) BulkUpdateLastAck(ctx context.Context, updates []*dto.UpdatesAck) error {
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
func (r *conversationRepo) UpdateSenderConversation(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, createdAt time.Time) error {
	db := r.getTx(ctx)
	return db.Model(&dao.Conversation{}).WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "owner_id"}, {Name: "conversation_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"other_user_id": receiverID,
			"updated_at":    time.Now(),
			"last_seq_id":   seqID,
			"last_ack_id":   seqID,
			"unread_count":  0,
		}), // 插入冲突时则进行更新操作
	}).Create(&dao.Conversation{
		OwnerID:        senderID,
		ConversationID: convID,
		OtherUserID:    receiverID,
		LastSeqID:      seqID,
		LastAckID:      seqID,
		UnreadCount:    0,
	}).Error
}

// UpdateReceiverConversation 更新接收者会话
func (r *conversationRepo) UpdateReceiverConversation(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, createdAt time.Time) error {
	db := r.getTx(ctx)
	return db.Model(&dao.Conversation{}).WithContext(ctx).Clauses(
		clause.OnConflict{
			Columns: []clause.Column{{Name: "owner_id"}, {Name: "conversation_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"other_user_id": senderID,
				"updated_at":    time.Now(),
				"last_seq_id":   seqID,
				"unread_count":  gorm.Expr("unread_count + ?", 1),
			}),
		}).Create(&dao.Conversation{
		OwnerID:        receiverID,
		ConversationID: convID,
		OtherUserID:    senderID,
		LastSeqID:      seqID,
		LastAckID:      0,
		UnreadCount:    1,
		UpdatedAt:      createdAt,
	}).Error
}
