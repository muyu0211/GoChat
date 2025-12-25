package repository

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// IBaseRepository 泛型接口，支持任意实体的一般操作
type IBaseRepository[T any] interface {
	getTx(ctx context.Context) *gorm.DB
	GetByID(ctx context.Context, id uint64) (*T, error)
	Create(ctx context.Context, entity *T) error
	Delete(ctx context.Context, entity *T) error
	Update(ctx context.Context, entity *T) error
	List(ctx context.Context, params QueryParams) ([]T, int64, error)
}

// QueryParams 查询参数
type QueryParams struct {
	Offset  int64
	Limit   int64
	OrderBy string
	Selects []string
	Query   []string
	Args    []interface{}
}

func isDuplicateError(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	return false
}
