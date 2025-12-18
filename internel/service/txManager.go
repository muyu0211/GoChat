package service

import (
	"GoChat/pkg/util"
	"context"
	"database/sql"
	"gorm.io/gorm"
)

// ITxManager 事务管理器接口
type ITxManager interface {
	ExecTx(ctx context.Context, fn svsFn, opts ...*sql.TxOptions) error
}

type TxManager struct {
	db *gorm.DB // 事务管理器同样持有一个数据库的原始连接，用于管理事务
}

type svsFn func(ctx context.Context) error // service层方法

func NewTxManager(db *gorm.DB) ITxManager {
	return &TxManager{db: db}
}

// ExecTx 闭包形式执行事务, 当有数据需要传递时,使用闭包变量
func (t *TxManager) ExecTx(ctx context.Context, fn svsFn, opts ...*sql.TxOptions) error {
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 将tx句柄存入ctx中
		txCtx := util.WithTx(ctx, tx)
		return fn(txCtx)
	}, opts...)
}
