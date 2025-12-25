package dao

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MessageModel struct {
	ID             uint64 `gorm:"primaryKey"` // 全局唯一 ID (自增或 Snowflake)
	ConversationID string `gorm:"type:varchar(32);index:idx_conversation_id;uniqueIndex:idx_conv_seq;not null;comment:会话id，使用会话双方id得到"`
	// ClientMsgID    string `gorm:"type:varchar(64);"`          // 客户端生成的UUID (用于去重)
	SeqID      uint64 `gorm:"index:idx_conversation_seq;uniqueIndex:idx_conv_seq"` // 会话内序列号 (严格递增)
	SenderID   uint64 `gorm:"type:bigint;not null;comment:发送方id"`
	ReceiverID uint64 `gorm:"type:bigint;not null;comment:接收方id（可以是user也可以是群）"`
	Content    string `gorm:"type:text;not null;comment:存放信息内容；可以是图片、音频、视频的url"`
	MsgType    byte   `gorm:"type:tinyint;not null;comment:消息类型"`
	MsgStatus  byte   `gorm:"type:tinyint;default:1;comment:消息状态：正常、撤回、删除"`
	IsPushed   bool   `gorm:"default:false;comment:消息是否推送给对方（对方是否已读）"`
	Extra      string `gorm:"type:text;comment:存放额外信息(可选)"`
	CreatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

func (mm MessageModel) TableName() string {
	return "message"
}

func MigrateMessage(db *gorm.DB) {
	err := db.AutoMigrate(&MessageModel{})
	if err != nil {
		zap.L().Warn("Message Table Create Warn:", zap.Error(err))
	}
	zap.L().Info("Message Table Migrate Success")
}
