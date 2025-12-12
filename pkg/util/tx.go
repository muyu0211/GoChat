package util

import (
	"context"
	"gorm.io/gorm"
)

// 将事务句柄存入ctx中
type ctxTxKey struct{}

func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, ctxTxKey{}, tx)
}

func GetTx(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(ctxTxKey{}).(*gorm.DB); ok {
		return tx
	}
	return nil
}
