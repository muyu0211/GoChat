package db

import (
	"GoChat/config"
	"GoChat/internel/model/dao"
	"GoChat/pkg/logger"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type MySQLConfig struct {
	Master          DBConfig      `mapstructure:"master"`
	Slave           DBConfig      `mapstructure:"slave"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int           `mapstructure:"conn_max_lifetime"`
	SlowThreshold   time.Duration `mapstructure:"slow_threshold"`
}

type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type DBS struct {
	Master *gorm.DB
	Slaves []*gorm.DB
}

var dbs *DBS

func GetDBS() *DBS {
	return dbs
}

func StartMySQL(cfg *config.Config) {
	var err error
	if dbs, err = initMySQL(&cfg.MySQLConfig); err != nil {
		panic(fmt.Errorf("mysql initialization failed, err: %v", err))
	}
	fmt.Println("===========mysql initialization successful!===========")
}

func initMySQL(sqlCfg *config.MySQLConfig) (*DBS, error) {
	var (
		MasterDB *gorm.DB
		SlaveDBs = make([]*gorm.DB, 0)
		err      error
	)
	if MasterDB, err = connDB(&sqlCfg.Master, sqlCfg); err != nil {
		return nil, err
	}
	// TODO: 以后可以创建从数据库

	zap.L().Info("数据库连接成功.")

	// 进行表迁移
	dao.MigrateUserBasic(MasterDB)
	dao.MigrateMessage(MasterDB)
	dao.MigrateConversation(MasterDB)
	dao.MigrateGroup(MasterDB)
	dao.MigrateGroupMember(MasterDB)
	dao.MigrateGroupMessage(MasterDB)

	return &DBS{
		Master: MasterDB,
		Slaves: SlaveDBs,
	}, nil
}

// connDB 创建数据库连接
func connDB(sqlCfg *config.DBConfig, commCfg *config.MySQLConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		sqlCfg.User,
		sqlCfg.Password,
		sqlCfg.Host,
		sqlCfg.Port,
		sqlCfg.DBName)

	gormLogger := logger.NewGormLogger(&config.Cfg.GormLoggerConfig)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})

	if err != nil {
		zap.L().Fatal("连接数据库失败", zap.String("error", err.Error()))
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(commCfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(commCfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(commCfg.ConnMaxLifetime) * time.Second)

	return db, nil
}

func CloseMySQL() {
	if dbs != nil {
		if dbs.Master != nil {
			d, _ := dbs.Master.DB()
			if err := d.Close(); err != nil {
				zap.L().Error("Master Mysql Close Failed: ", zap.Error(err))
			}
		}
		for idx, slave := range dbs.Slaves {
			if slave != nil {
				d, _ := slave.DB()
				if err := d.Close(); err != nil {
					zap.L().Error(fmt.Sprintf("Slave:[%d] Mysql Close Failed: %v", idx, zap.Error(err)))
				}
			}
		}
	}
}
