package gormdb

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/LazyEasyDev/LZApp/config"
	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func Init(ctx context.Context, appConfig *config.AppConfig) (*gorm.DB, error) {
	logMode := gormlogger.Warn
	if appConfig.Profile == config.ProfileDebug {
		logMode = gormlogger.Info
	}

	database, err := gorm.Open(mysql.Open(dataSourceName(appConfig.DB)), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logMode),
	})
	if err != nil {
		return nil, fmt.Errorf("open MySQL: %w", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("get SQL database: %w", err)
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping MySQL: %w", err)
	}
	return database, nil
}

func Close(database *gorm.DB) error {
	if database == nil {
		return nil
	}
	sqlDB, err := database.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func SQLDB(database *gorm.DB) (*sql.DB, error) {
	if database == nil {
		return nil, fmt.Errorf("GORM database is required")
	}
	return database.DB()
}

func dataSourceName(dbConfig *config.DBConfig) string {
	driverConfig := mysqldriver.NewConfig()
	driverConfig.User = dbConfig.User
	driverConfig.Passwd = dbConfig.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(dbConfig.Host, fmt.Sprintf("%d", dbConfig.Port))
	driverConfig.DBName = dbConfig.DBName
	driverConfig.ParseTime = true
	driverConfig.Loc = time.UTC
	driverConfig.Timeout = 5 * time.Second
	driverConfig.ReadTimeout = 10 * time.Second
	driverConfig.WriteTimeout = 10 * time.Second
	driverConfig.Params = map[string]string{"charset": dbConfig.Charset}
	return driverConfig.FormatDSN()
}
