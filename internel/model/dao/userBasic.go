package dao

import (
	"go.uber.org/zap"
	"gorm.io/gorm"
	"time"
)

type UserState byte

const (
	Admin UserState = iota
	UserStateActive
	UserStateBanned
	UserStateDeleted
)

type UserBasicModel struct {
	ID             uint64         `gorm:"primaryKey autoIncrement;comment:用户ID（自增）"`
	Email          *string        `gorm:"type:varchar(20);not null;uniqueIndex:idx_email;comment:邮箱号（唯一，脱敏存储）"`
	Phone          *string        `gorm:"type:varchar(20);uniqueIndex:idx_phone;comment:手机号（唯一，脱敏存储）"`
	PassWordHashed string         `gorm:"type:varchar(100);not null;comment:密码（bcrypt加密）"`
	Nickname       string         `gorm:"type:varchar(50);default:'';comment:昵称"`
	Age            uint8          `gorm:"type:tinyint unsigned;default:18;comment:年龄（0-255）"`
	Gender         uint8          `gorm:"type:tinyint;default:0;comment:性别（0-未知，1-男，2-女）"`
	State          UserState      `gorm:"type:tinyint;default:1;comment:状态（0-管理员，1-正常，2-禁用）"`
	Avatar         string         `gorm:"type:varchar(255);default:'';comment:头像URL"`
	CreatedAt      time.Time      `gorm:"type:datetime;comment:创建时间"`
	UpdatedAt      time.Time      `gorm:"type:datetime;comment:更新时间"`
	DeletedAt      gorm.DeletedAt `gorm:"softDelete:flag;comment:删除时间（软删除）"`
}

func (u UserBasicModel) TableName() string {
	return "user_basic"
}

func MigrateUserBasic(db *gorm.DB) {
	err := db.AutoMigrate(&UserBasicModel{})
	if err != nil {
		zap.L().Warn("User Table Create Warn:", zap.Error(err))
	}
	zap.L().Info("User Table Migrate Success")
}
