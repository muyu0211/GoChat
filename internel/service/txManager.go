package service

import (
	"GoChat/pkg/util"
	"context"
	"database/sql"

	"gorm.io/gorm"
)

// ITxManager 事务管理器接口
type ITxManager interface {
	ExecTx(ctx context.Context, fn func(ctx context.Context) error, opts ...*sql.TxOptions) error
}

type TxManager struct {
	db *gorm.DB // 事务管理器同样持有一个数据库的原始连接，用于管理事务
}

func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{db: db}
}

// ExecTx 闭包形式执行事务, 当有数据需要传递时,使用闭包变量
func (t *TxManager) ExecTx(ctx context.Context, fn func(ctx context.Context) error, opts ...*sql.TxOptions) error {
	// 将ctx绑定至事务句柄tx
	tx := t.db.WithContext(ctx)
	// 执行事务
	err := tx.Transaction(func(tx *gorm.DB) error {
		txCtx := withTx(ctx, tx) // 将 tx 句柄存入 ctx 中
		return fn(txCtx)
	}, opts...)
	return err
}

func withTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, util.CtxTxKey{}, tx)
}
