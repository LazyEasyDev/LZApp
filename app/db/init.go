package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LazyEasyDev/LZApp/app/users"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/LazyEasyDev/LZApp/config"
)

func Run(ctx context.Context) (runErr error) {
	appConfig := config.GetConfig()
	if appConfig == nil || appConfig.DB == nil || !appConfig.DB.Enabled {
		return fmt.Errorf("database is disabled or not configured")
	}
	// Initialize the database component
	init_err := components.InitDB(ctx, appConfig)
	if init_err != nil {
		return fmt.Errorf("initialize database: %w", init_err)
	}
	defer func() {
		if err := components.CloseDB(); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}()

	// Set up the necessary tables and initial data.
	return createTables(ctx)
}

func createTables(ctx context.Context) error {
	// Get the database instance from the components runtime
	database := components.GetComponents().DB
	// Create the tables
	if err := users.CreateTable(ctx, database); err != nil {
		return fmt.Errorf("create user table: %w", err)
	}
	// Create the data
	if err := users.InitData(ctx, database); err != nil {
		return fmt.Errorf("initialize user data: %w", err)
	}
	//
	slog.Info("user table initialized successfully")
	return nil
}
