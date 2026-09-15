package db

import (
	"context"
	"fmt"

	"github.com/LazyEasyDev/LZApp/src/user_manager"
	"gorm.io/gorm"
)

func Init(ctx context.Context, database *gorm.DB) error {
	if err := usermanager.CreateTable(ctx, database); err != nil {
		return fmt.Errorf("create user table: %w", err)
	}
	if err := usermanager.InitData(ctx, database); err != nil {
		return fmt.Errorf("initialize user data: %w", err)
	}
	return nil
}
