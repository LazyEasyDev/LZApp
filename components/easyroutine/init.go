package easyroutine

import (
	"context"
	"fmt"

	easyroutinelib "github.com/LazyEasyDev/EasyRoutine"
	"github.com/LazyEasyDev/LZApp/components/gormdb"
	"gorm.io/gorm"
)

func Init(ctx context.Context, database *gorm.DB) error {
	sqlDB, err := gormdb.SQLDB(database)
	if err != nil {
		return fmt.Errorf("get SQL database: %w", err)
	}
	if err := easyroutinelib.InitSQLLease(ctx, sqlDB, easyroutinelib.SQLMySQL); err != nil {
		return fmt.Errorf("initialize MySQL lease: %w", err)
	}
	return nil
}
