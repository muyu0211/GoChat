package dao

import (
	"go.uber.org/zap"
	"gorm.io/gorm"
	"time"
)

// ConversationModel 代表用户的一个聊天会话 (例如 UserA 和 UserB 的聊天)
type ConversationModel struct {
	ID             uint64 `gorm:"primaryKey"`
	OwnerID        string `gorm:"type:varchar(32);index:idx_owner_updated;comment:属于谁"`
	ConversationID string `gorm:"type:varchar(64);index;comment:会话ID (A_B)"`
	OtherUserID    string `gorm:"type:varchar(32);comment:对方ID"`
	LastSeqID      uint64 `gorm:"default:0;comment:最新一条消息的SeqID"`
	LastAckID      uint64 `gorm:"default:0;comment:最后确认接收的SeqID"`
	UnreadCount    uint64 `gorm:"default:0;comment:未读数"`
	UpdatedAt      time.Time
}

func (cm ConversationModel) TableName() string {
	return "conversation"
}

func MigrateConversation(db *gorm.DB) {
	err := db.AutoMigrate(&ConversationModel{})
	if err != nil {
		zap.L().Warn("Conversation Table Create Warn:", zap.Error(err))
	}
	zap.L().Info("Conversation Table Migrate Success")
}
