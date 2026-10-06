package gormdb

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/LazyEasyDev/LZApp/config/db_config"
	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	maxIdleConns    = 5
	maxOpenConns    = 25
	connMaxIdleTime = 5 * time.Minute
	connMaxLifetime = 30 * time.Minute

	slowQueryThreshold = 200 * time.Millisecond

	dialTimeout  = 5 * time.Second
	readTimeout  = 30 * time.Second
	writeTimeout = 30 * time.Second

	defaultLogMode = gormlogger.Warn
)

func New(ctx context.Context, dbConfig *db_config.DBConfig) (*gorm.DB, error) {

	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}

	if dbConfig == nil {
		return nil, fmt.Errorf("database configuration is required")
	}

	logMode := defaultLogMode
	switch dbConfig.LogLevel {
	case "silent":
		logMode = gormlogger.Silent
	case "error":
		logMode = gormlogger.Error
	case "warn":
		logMode = gormlogger.Warn
	case "info":
		logMode = gormlogger.Info
	default:
		logMode = defaultLogMode
	}

	database, err := gorm.Open(mysql.Open(dataSourceName(dbConfig)), &gorm.Config{
		Logger: newLogger(logMode),
	})
	if err != nil {
		return nil, fmt.Errorf("open MySQL: %w", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("get SQL database: %w", err)
	}
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping MySQL: %w", err)
	}
	return database, nil
}

func newLogger(level gormlogger.LogLevel) gormlogger.Interface {
	return gormlogger.NewSlogLogger(slog.Default(), gormlogger.Config{
		LogLevel:             level,
		SlowThreshold:        slowQueryThreshold,
		ParameterizedQueries: true,
	})
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

func dataSourceName(dbConfig *db_config.DBConfig) string {
	driverConfig := mysqldriver.NewConfig()
	driverConfig.User = dbConfig.User
	driverConfig.Passwd = dbConfig.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(dbConfig.Host, fmt.Sprintf("%d", dbConfig.Port))
	driverConfig.DBName = dbConfig.DBName
	driverConfig.ParseTime = true
	driverConfig.Loc = time.UTC
	driverConfig.Timeout = dialTimeout
	driverConfig.ReadTimeout = readTimeout
	driverConfig.WriteTimeout = writeTimeout
	driverConfig.Params = map[string]string{"charset": dbConfig.Charset}
	return driverConfig.FormatDSN()
}
