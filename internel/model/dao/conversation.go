package dao

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ConversationModel 代表用户的一个聊天会话 (例如 UserA 和 UserB 的聊天)
type ConversationModel struct {
	ID             uint64 `gorm:"primaryKey"`
	OwnerID        uint64 `gorm:"index:idx_owner_updated;uniqueIndex:idx_owner_conv;comment:属于谁"`
	ConversationID string `gorm:"index:idx_conv_id;uniqueIndex:idx_owner_conv;comment:会话ID (A_B)"`
	OtherUserID    uint64 `gorm:"comment:对方ID"`
	LastSeqID      uint64 `gorm:"default:0;comment:最新一条消息的SeqID"`
	LastAckID      uint64 `gorm:"default:0;comment:最后确认接收的SeqID"`
	UnreadCount    uint64 `gorm:"default:0;comment:未读数"`
	IsPinned       bool   `gorm:"default:false;comment:是否置顶"`
	IsMuted        bool   `gorm:"default:false;comment:是否免打扰"`
	IsDeleted      bool   `gorm:"default:false;comment:是否已经删除好友（拉黑）"`
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt
}

func (cm ConversationModel) TableName() string {
	return "conversation"
}

func MigrateConversation(db *gorm.DB) {
	err := db.AutoMigrate(&ConversationModel{})
	if err != nil {
		zap.L().Warn("ConversationModel Table Create Warn:", zap.Error(err))
	}
}
