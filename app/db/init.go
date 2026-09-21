package db

import (
	"context"
	"fmt"
	"log/slog"

	usermanager "github.com/LazyEasyDev/LZApp/app/user_manager"
	"gorm.io/gorm"
)

func Init(ctx context.Context, database *gorm.DB) error {
	if err := usermanager.CreateTable(ctx, database); err != nil {
		return fmt.Errorf("create user table: %w", err)
	}
	if err := usermanager.InitData(ctx, database); err != nil {
		return fmt.Errorf("initialize user data: %w", err)
	}
	slog.Info("user table initialized successfully")
	return nil
}
