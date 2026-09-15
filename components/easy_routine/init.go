package easyroutine

import (
	"context"
	"fmt"

	EasyRoutine "github.com/LazyEasyDev/EasyRoutine"
	gormdb "github.com/LazyEasyDev/LZApp/components/gorm_db"
	"gorm.io/gorm"
)

func Init(ctx context.Context, database *gorm.DB) error {
	sqlDB, err := gormdb.SQLDB(database)
	if err != nil {
		return fmt.Errorf("get SQL database: %w", err)
	}
	if err := EasyRoutine.InitSQLLease(ctx, sqlDB, EasyRoutine.SQLMySQL); err != nil {
		return fmt.Errorf("initialize MySQL lease: %w", err)
	}
	return nil

}
