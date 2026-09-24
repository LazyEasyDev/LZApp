package gormdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/LazyEasyDev/LZApp/config"
)

func EnsureDatabase(ctx context.Context, dbConfig *config.DBConfig) (ensureErr error) {
	if dbConfig == nil || dbConfig.DBName == "" {
		return fmt.Errorf("database name is required")
	}

	serverConfig := *dbConfig
	serverConfig.DBName = ""
	database, err := sql.Open("mysql", dataSourceName(&serverConfig))
	if err != nil {
		return fmt.Errorf("open MySQL server: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			ensureErr = errors.Join(ensureErr, fmt.Errorf("close MySQL server connection: %w", err))
		}
	}()

	return ensureDatabase(ctx, database, dbConfig)
}

func ensureDatabase(ctx context.Context, database *sql.DB, dbConfig *config.DBConfig) error {
	var databaseName string
	err := database.QueryRowContext(ctx,
		"SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?",
		dbConfig.DBName,
	).Scan(&databaseName)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check database %q: %w", dbConfig.DBName, err)
	}

	statement := "CREATE DATABASE IF NOT EXISTS `" + strings.ReplaceAll(dbConfig.DBName, "`", "``") + "`"
	if dbConfig.Charset != "" {
		statement += " CHARACTER SET `" + strings.ReplaceAll(dbConfig.Charset, "`", "``") + "`"
	}
	if _, err := database.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("create database %q: %w", dbConfig.DBName, err)
	}
	return nil
}
