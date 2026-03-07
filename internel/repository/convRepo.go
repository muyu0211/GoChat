package repository

import (
	"GoChat/internel/model/dao"
	"GoChat/internel/model/dto"
	"GoChat/pkg/util"
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IConvRepo interface {
	IBaseRepository[dao.ConversationModel]
	CreateConvBatch(ctx context.Context, entities []dao.ConversationModel, batchSize int) error
	GetConvByUserID(ctx context.Context, userID uint64) ([]dao.ConversationModel, error)
	GetConvByConvID(ctx context.Context, conversationID string) (*dao.ConversationModel, error)
	GetConvByUserIDConvID(ctx context.Context, userID uint64, conversationIDs []string) ([]dao.ConversationModel, error)
	GetLastSeqID(ctx context.Context, userID uint64, conversationID string) (uint64, error)
	UpdateLastAck(ctx context.Context, userID uint64, conversationID string, lastAckID uint64) error
	BulkUpdateLastAck(ctx context.Context, updates []*dto.UpdatesAck) error
	UpdateSenderConversation(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, createdAt time.Time) error
	UpdateReceiverConversation(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, createdAt time.Time) error
	UpdateBothConversations(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, createdAt time.Time) error
	UpdateGroupConversations(ctx context.Context, groupKeyID string, memberIDs []uint64, newSeq uint64, senderID uint64) error
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
func (r *ConvRepo) GetByID(ctx context.Context, id uint64) (*dao.ConversationModel, error) {
	return nil, nil
}
func (r *ConvRepo) Create(ctx context.Context, entity *dao.ConversationModel) error {
	return nil
}
func (r *ConvRepo) Delete(ctx context.Context, entity *dao.ConversationModel) error {
	return nil
}
func (r *ConvRepo) Update(ctx context.Context, entity *dao.ConversationModel) error {
	return nil
}
func (r *ConvRepo) List(ctx context.Context, params QueryParams) ([]dao.ConversationModel, int64, error) {
	return nil, 0, nil
}

func (r *ConvRepo) CreateConvBatch(ctx context.Context, entities []dao.ConversationModel, batchSize int) error {
	db := r.getTx(ctx).WithContext(ctx)
	if err := db.Model(&dao.ConversationModel{}).CreateInBatches(entities, batchSize).Error; err != nil {
		return err
	}
	return nil
}

// GetConvByUserID 根据UserID获取该用户的所有会话
func (r *ConvRepo) GetConvByUserID(ctx context.Context, userID uint64) ([]dao.ConversationModel, error) {
	db := r.getTx(ctx)
	var conversations []dao.ConversationModel
	result := db.Model(&dao.ConversationModel{}).WithContext(ctx).
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

func (r *ConvRepo) GetConvByConvID(ctx context.Context, conversationID string) (*dao.ConversationModel, error) {
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

// GetConvByUserIDConvID 根据UserID和ConvID批量获取会话
func (r *ConvRepo) GetConvByUserIDConvID(ctx context.Context, userID uint64, conversationIDs []string) ([]dao.ConversationModel, error) {
	db := r.getTx(ctx)
	var conversations []dao.ConversationModel
	result := db.Model(&dao.ConversationModel{}).WithContext(ctx).
		Where("user_id = ? and conversation_id in ?", userID, conversationIDs).
		Find(&conversations)

	if result.Error != nil {
		return nil, result.Error
	}
	return conversations, nil
}

func (r *ConvRepo) GetLastSeqID(ctx context.Context, userID uint64, conversationID string) (uint64, error) {
	db := r.getTx(ctx)
	var conversation dao.ConversationModel
	result := db.Model(&dao.ConversationModel{}).WithContext(ctx).
		Select("last_seq_id").
		Where("owner_id = ? and conversation_id = ?", userID, conversationID).Find(&conversation)
	if result.Error != nil {
		return 0, result.Error
	}
	return conversation.LastSeqID, nil
}

func (r *ConvRepo) UpdateLastAck(ctx context.Context, userID uint64, conversationID string, lastAckID uint64) error {
	db := r.getTx(ctx)
	result := db.Model(&dao.ConversationModel{}).WithContext(ctx).
		Where("owner_id = ? and conversation_id = ?", userID, conversationID).
		Updates(map[string]interface{}{
			"last_ack_id":  lastAckID,
			"unread_count": 0,
		})
	return result.Error
}

func (r *ConvRepo) BulkUpdateLastAck(ctx context.Context, updates []*dto.UpdatesAck) error {
	if len(updates) == 0 {
		return nil
	}
	db := r.getTx(ctx)
	// 拼接sql
	var sql strings.Builder
	args := make([]interface{}, 0, 5*len(updates))

	// 1. 构建CASE语句更新last_ack_id
	sql.WriteString("UPDATE conversation SET last_ack_id = CASE")
	for _, item := range updates {
		sql.WriteString("WHEN owner_id = ? AND conversation_id = ? THEN ?")
		args = append(args, item.UserID, item.ConversationID, item.LastAckID)
	}
	sql.WriteString(" ELSE last_ack_id END")
	sql.WriteString(", unread_count = CASE")
	for _, item := range updates {
		sql.WriteString(" WHEN owner_id = ? AND conversation_id = ? THEN 0")
		args = append(args, item.UserID, item.ConversationID)
	}
	sql.WriteString(" ELSE unread_count END")

	// 3. 构建 WHERE 条件
	sql.WriteString(" WHERE (")
	for j, item := range updates {
		if j > 0 {
			sql.WriteString(" OR ")
		}
		sql.WriteString("(owner_id = ? AND conversation_id = ?)")
		args = append(args, item.UserID, item.ConversationID)
	}
	sql.WriteString(")")

	if err := db.Model(&dao.ConversationModel{}).WithContext(ctx).Exec(sql.String(), args...).Error; err != nil {
		return err
	}
	return nil
}

// UpdateSenderConversation 更新发送者会话
func (r *ConvRepo) UpdateSenderConversation(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, updatedAt time.Time) error {
	db := r.getTx(ctx)
	return db.Model(&dao.ConversationModel{}).WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "owner_id"}, {Name: "conversation_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"updated_at":   updatedAt,
			"last_seq_id":  gorm.Expr("GREATEST(last_seq_id, ?)", seqID),
			"last_ack_id":  gorm.Expr("GREATEST(last_ack_id, ?)", seqID),
			"unread_count": 0,
		}), // 插入冲突时则进行更新操作
	}).Create(&dao.ConversationModel{
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
	return db.Model(&dao.ConversationModel{}).WithContext(ctx).Clauses(
		clause.OnConflict{
			Columns: []clause.Column{{Name: "owner_id"}, {Name: "conversation_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"updated_at":   updatedAt,
				"last_seq_id":  gorm.Expr("GREATEST(last_seq_id , ?)", seqID),
				"unread_count": gorm.Expr("unread_count + ?", 1),
			}),
		}).Create(&dao.ConversationModel{
		OwnerID:        receiverID,
		ConversationID: convID,
		OtherUserID:    senderID,
		LastSeqID:      seqID,
		LastAckID:      0,
		UnreadCount:    1,
		UpdatedAt:      updatedAt,
	}).Error
}

// UpdateBothConversations 一次性更新发送方和接收方的会话（按固定顺序，避免死锁）
func (r *ConvRepo) UpdateBothConversations(ctx context.Context, senderID, receiverID uint64, convID string, seqID uint64, updatedAt time.Time) error {
	// 按固定顺序更新：总是先更新用户ID较小的，再更新用户ID较大的
	smallID := senderID
	largeID := receiverID
	if smallID > largeID {
		smallID, largeID = largeID, smallID
	}

	// 先更新小ID的会话
	if smallID == senderID {
		// 小ID是发送方，先更新发送方，再更新接收方
		if err := r.UpdateSenderConversation(ctx, senderID, receiverID, convID, seqID, updatedAt); err != nil {
			return err
		}
		if err := r.UpdateReceiverConversation(ctx, senderID, receiverID, convID, seqID, updatedAt); err != nil {
			return err
		}
	} else {
		// 小ID是接收方，先更新接收方，再更新发送方
		if err := r.UpdateReceiverConversation(ctx, senderID, receiverID, convID, seqID, updatedAt); err != nil {
			return err
		}
		if err := r.UpdateSenderConversation(ctx, senderID, receiverID, convID, seqID, updatedAt); err != nil {
			return err
		}
	}

	return nil
}

func (r *ConvRepo) UpdateGroupConversations(ctx context.Context, groupKeyID int64, memberIDs []uint64, newSeq uint64, senderID uint64) error {
	// 1. 更新所有接收者 (Unread + 1)
	db := r.getTx(ctx).WithContext(ctx)
	err := db.Model(&dao.ConversationModel{}).
		Where("conversation_id = ? AND owner_id IN (?) AND owner_id != ?", strconv.FormatInt(groupKeyID, 10), memberIDs, senderID).
		Updates(map[string]interface{}{
			"last_seq_id":  newSeq,
			"updated_at":   time.Now(),
			"unread_count": gorm.Expr("unread_count + 1"),
		}).Error

	// 2. 更新发送者 (Unread 不变, LastAck 追平)
	db.Model(&dao.ConversationModel{}).
		Where("conversation_id = ? AND owner_id = ?", strconv.FormatInt(groupKeyID, 10), senderID).
		Updates(map[string]interface{}{
			"last_seq_id": newSeq,
			"last_ack_id": newSeq,
			"updated_at":  time.Now(),
		})

	if err != nil {
		zap.L().Error("更新 conversation 信息失败", zap.Error(err), zap.Int64("groupKeyID", groupKeyID))
	}

	return err
}
