package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LazyEasyDev/LZApp/app/users"
	"github.com/LazyEasyDev/LZApp/components/gormdb"
	"github.com/LazyEasyDev/LZApp/config"
	"gorm.io/gorm"
)

func Run(ctx context.Context) (runErr error) {
	appConfig := config.GetConfig()
	if appConfig == nil || appConfig.DB == nil || !appConfig.DB.Enabled {
		return fmt.Errorf("database is disabled or not configured")
	}
	database, err := gormdb.Init(ctx, appConfig)
	if err != nil {
		return err
	}
	defer func() {
		runErr = errors.Join(runErr, gormdb.Close(database))
	}()
	return Init(ctx, database)
}

func Init(ctx context.Context, database *gorm.DB) error {
	if err := users.CreateTable(ctx, database); err != nil {
		return fmt.Errorf("create user table: %w", err)
	}
	if err := users.InitData(ctx, database); err != nil {
		return fmt.Errorf("initialize user data: %w", err)
	}
	slog.Info("user table initialized successfully")
	return nil
}
