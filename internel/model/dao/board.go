package dao

import (
	"go.uber.org/zap"
	"gorm.io/gorm"
	"time"
)

type Board struct {
	BoardID string `gorm:"primaryKey;type:varchar(15);comment:白板的唯一标识（15位长的字符串）"`
	OwnerID uint64 `gorm:"index;not null;comment:所有者ID"`
	Title   string `gorm:"type:varchar(50);default:'无标题白板'"`

	// 核心状态字段
	IsShared  bool   `gorm:"default:false"`          // 是否开启了分享 进行权限控制,
	ShareCode string `gorm:"type:varchar(20);index"` // 分享码 (类似于百度网盘密码)

	// 数据快照 - 用于冷存储
	Snapshot *[]byte `gorm:"type:mediumblob;comment:存储白板原信息"` // TODO: 感觉可以考虑对元信息分表，此处存放一个压缩后的信息，元信息单独存放

	CreatedAt time.Time      `gorm:"type:datetime;comment:创建时间"`
	UpdatedAt time.Time      `gorm:"type:datetime;comment:更新时间"`
	DeletedAt gorm.DeletedAt `gorm:"comment:删除时间（软删除）"`
}

func (Board) TableName() string {
	return "boards"
}

func MigrateBoard(db *gorm.DB) {
	err := db.AutoMigrate(&Board{})
	if err != nil {
		zap.L().Warn("Board Table Create Warn:", zap.Error(err))
	}
	zap.L().Info("Board Table Migrate Success")
}
