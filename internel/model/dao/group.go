package dao

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Group 群组主表（存储群的元数据）
type Group struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement:false"` // 雪花算法ID, 类似于QQ群号
	Name         string `gorm:"type:varchar(64);not null"`      // 群名称
	Avatar       string `gorm:"type:varchar(255)"`              // 群头像
	OwnerID      uint64 `gorm:"index;not null"`                 // 群主ID (冗余字段，查询方便)
	Type         int    `gorm:"default:1"`                      // 群类型: 1-普通群, 2-超大群(直播群)
	MaxMembers   int    `gorm:"default:500"`                    // 最大人数限制
	IsMuteAll    bool   `gorm:"default:false"`                  // 是否全员禁言
	JoinType     int    `gorm:"default:0"`                      // 加群方式: 0-自由加入, 1-需验证, 2-禁止加入
	Notification string `gorm:"type:varchar(1024)"`             // 群公告
	InfoSeq      int64  `gorm:"default:0"`                      // 群资料的版本号 (每次修改资料+1)
	Status       int    `gorm:"default:1"`                      // 状态: 1-正常, 2-解散, 3-封禁
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type GroupMember struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`                   // 自增ID (内部管理用)
	GroupID     uint64    `gorm:"uniqueIndex:idx_group_user;index:idx_group"` // 群ID
	UserID      uint64    `gorm:"uniqueIndex:idx_group_user;index:idx_user"`  // 用户ID
	Role        int       `gorm:"default:1"`                                  // 角色: 1-普通成员, 2-管理员, 3-群主
	MuteEndTime int64     `gorm:"default:0"`                                  // 禁言截止时间戳 (0代表不禁言)
	Nickname    string    `gorm:"type:varchar(64)"`                           // 我在群里的昵称
	IsDisturb   bool      `gorm:"default:false"`                              // 是否免打扰
	IsPinned    bool      `gorm:"default:false"`                              // 是否置顶该群
	JoinTime    time.Time `gorm:"autoCreateTime"`                             // 入群时间
	JoinSource  int       `gorm:"default:0"`                                  // 入群方式: 0-扫码, 1-邀请, 2-搜索
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type GroupMessage struct {
	ID         uint64 `gorm:"primaryKey"`          // 全局唯一消息ID
	GroupID    uint64 `gorm:"index:idx_group_seq"` // 群ID
	SeqID      uint64 `gorm:"index:idx_group_seq"` // 群内单调递增序列号 (核心)
	SenderID   uint64 `gorm:"index"`               // 发送者
	Content    string `gorm:"type:text"`           // 消息内容
	Type       int    `gorm:"default:1"`           // 消息类型
	AtUserList string `gorm:"type:varchar(1024)"`  // JSON: [1001, 1002] 或 "all"
	CreatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

func (g Group) TableName() string {
	return "group"
}

func (gm GroupMember) TableName() string {
	return "group_member"
}

func (gm GroupMessage) TableName() string {
	return "group_message"
}

func MigrateGroup(db *gorm.DB) {
	if err := db.AutoMigrate(&Group{}); err != nil {
		zap.L().Warn("Conversation Table Create Warn:", zap.Error(err))
	}
}

func MigrateGroupMember(db *gorm.DB) {
	if err := db.AutoMigrate(&GroupMember{}); err != nil {
		zap.L().Warn("Conversation Table Create Warn:", zap.Error(err))
	}
}

func MigrateGroupMessage(db *gorm.DB) {
	if err := db.AutoMigrate(&GroupMessage{}); err != nil {
		zap.L().Warn("Conversation Table Create Warn:", zap.Error(err))
	}
}
