package components

import (
	"context"
	"fmt"
	"log/slog"

	cachelib "github.com/LazyEasyDev/LCache"
	easylog "github.com/LazyEasyDev/LZApp/components/easy_log"
	easyroutine "github.com/LazyEasyDev/LZApp/components/easy_routine"
	gormdb "github.com/LazyEasyDev/LZApp/components/gorm_db"
	"github.com/LazyEasyDev/LZApp/components/lcache"
	"github.com/LazyEasyDev/LZApp/config"
	"gorm.io/gorm"
)

type Runtime struct {
	DB    *gorm.DB
	Cache *cachelib.Cache
}

var fundementalInitialized bool
var runtimeInitialized bool
var runtime *Runtime

func InitFundemental(appConfig *config.AppConfig) error {
	if fundementalInitialized {
		return nil
	}
	if err := easylog.Init(appConfig.Log); err != nil {
		return fmt.Errorf("initialize logging: %w", err)
	}
	runtime = &Runtime{}
	runtime.Cache = lcache.Init(appConfig.Cache)
	fundementalInitialized = true
	return nil
}

func Init(ctx context.Context, appConfig *config.AppConfig) error {

	if runtimeInitialized {
		return nil
	}

	if appConfig.DB.Enabled {
		database, err := gormdb.Init(ctx, appConfig)
		if err != nil {
			return fmt.Errorf("initialize database: %w", err)
		}
		runtime.DB = database

		if appConfig.EasyRoutine.Enabled {
			if err := easyroutine.Init(ctx, database); err != nil {
				return fmt.Errorf("initialize routine coordinator: %w", err)
			}
		}
	}

	if appConfig.HTTP.Enabled {
		// Initialize HTTP server or related components here
	}

	//
	runtimeInitialized = true
	slog.Info("components initialized")
	return nil
}

func Close() {
	appConfig := config.GetConfig()
	if runtimeInitialized {
		if appConfig.DB.Enabled {
			gormdb.Close(runtime.DB)
		}
		//
		if appConfig.HTTP.Enabled {

		}
	}
	slog.Info("components closed")
}
