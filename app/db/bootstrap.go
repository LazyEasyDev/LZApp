package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LazyEasyDev/LZApp/app/users"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/LazyEasyDev/LZApp/components/gormdb"
	"github.com/LazyEasyDev/LZApp/config"
)

func Run(ctx context.Context) (runErr error) {
	appConfig := config.GetConfig()
	if appConfig == nil {
		return fmt.Errorf("application configuration is required")
	}
	if err := components.InitSecurity(appConfig.SecurityHMAC, appConfig.SecurityBcrypt); err != nil {
		return fmt.Errorf("initialize security: %w", err)
	}
	// Ensure the database exists before initializing it.
	if err := gormdb.EnsureDatabase(ctx, appConfig.DB); err != nil {
		return err
	}
	// Initialize the database component
	initErr := components.InitDB(ctx, appConfig.DB)
	if initErr != nil {
		return initErr
	}
	defer func() {
		if err := components.CloseDB(); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}()

	// Set up the necessary tables and initial data.
	slog.Info("creating tables in the database")
	return createTables(ctx)
}

func createTables(ctx context.Context) error {
	// Create the tables
	if err := users.CreateTable(ctx); err != nil {
		return fmt.Errorf("create user table: %w", err)
	}
	// Create the data
	if err := users.InitData(ctx); err != nil {
		return fmt.Errorf("initialize user data: %w", err)
	}
	//
	slog.Info("database tables initialized successfully")
	return nil
}
