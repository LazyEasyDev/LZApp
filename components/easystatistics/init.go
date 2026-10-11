package easystatistics

import (
	"database/sql"
	"fmt"

	"github.com/LazyEasyDev/EasyStatistics"
)

func Init(sqlDB *sql.DB) error {
	if sqlDB == nil {
		return fmt.Errorf("SQL database is nil")
	}
	if err := EasyStatistics.Init(sqlDB, EasyStatistics.SQLMySQL); err != nil {
		return fmt.Errorf("initialize MySQL EasyStatistics: %w", err)
	}
	return nil
}

func Close() error {
	return EasyStatistics.Close()
}
