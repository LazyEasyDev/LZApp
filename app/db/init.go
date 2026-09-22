package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/LazyEasyDev/LZApp/app/users"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/LazyEasyDev/LZApp/config"
	"gorm.io/gorm"
)

func Run(ctx context.Context) (runErr error) {
	appConfig := config.GetConfig()
	if appConfig == nil || appConfig.DB == nil || !appConfig.DB.Enabled {
		return fmt.Errorf("database is disabled or not configured")
	}
	// Initialize the database component
	db, init_err := components.InitDB(ctx, appConfig)
	if init_err != nil {
		return fmt.Errorf("initialize database: %w", init_err)
	}
	// Set up the necessary tables and initial data.
	return Init(ctx, db)
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
