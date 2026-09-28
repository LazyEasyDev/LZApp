package easyroutine

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LazyEasyDev/EasyRoutine"
)

func Init(ctx context.Context, sqlDB *sql.DB) error {

	if sqlDB == nil {
		return fmt.Errorf("SQL database is nil")
	}

	if err := EasyRoutine.InitSQLLease(ctx, sqlDB, EasyRoutine.SQLMySQL); err != nil {
		return fmt.Errorf("initialize MySQL lease: %w", err)
	}
	return nil
}
