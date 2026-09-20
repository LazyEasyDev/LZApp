package components

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LazyEasyDev/EasyRoutine"
	cachelib "github.com/LazyEasyDev/LCache"

	easylog "github.com/LazyEasyDev/LZApp/components/easy_log"
	easyroutine "github.com/LazyEasyDev/LZApp/components/easy_routine"
	gormdb "github.com/LazyEasyDev/LZApp/components/gorm_db"
	httpserver "github.com/LazyEasyDev/LZApp/components/http_server"
	"github.com/LazyEasyDev/LZApp/components/lcache"
	"github.com/LazyEasyDev/LZApp/config"
	"gorm.io/gorm"
)

type Runtime struct {
	DB     *gorm.DB
	LCache *cachelib.Cache
	HTTP   *httpserver.Server
}

var runtime Runtime

func GetComponents() Runtime {
	return runtime
}

/*
	Init can never be called more than once or by multiple goroutines simultaneously.
	Any Error return will be followed by Close being called.
	Any Error will result in system termination.
*/

func Init(ctx context.Context, appConfig *config.AppConfig) error {
	// Initialize the logging system
	if err := easylog.Init(appConfig.Log); err != nil {
		slog.Error("failed to initialize logging:" + err.Error())
		return fmt.Errorf("initialize logging: %w", err)
	}
	// Initialize the local cache
	runtime.LCache = lcache.Init(appConfig.Cache)

	if appConfig.DB.Enabled {
		// Initialize the database if enabled
		database, err := gormdb.Init(ctx, appConfig)
		if err != nil {
			slog.Error("failed to initialize database:" + err.Error())
			return fmt.Errorf("initialize database: %w", err)
		}
		runtime.DB = database

		// Initialize the routine coordinator if enabled
		if appConfig.EasyRoutine.Enabled {
			if err := easyroutine.Init(ctx, runtime.DB); err != nil {
				slog.Error("failed to initialize routine coordinator:" + err.Error())
				return fmt.Errorf("initialize routine coordinator: %w", err)
			}
		}
	}

	// Initialize the HTTP server if enabled
	if appConfig.HTTP.Enabled {
		server, err := httpserver.Init(appConfig.HTTP)
		if err != nil {
			slog.Error("failed to initialize HTTP server:" + err.Error())
			return fmt.Errorf("initialize HTTP server: %w", err)
		}
		runtime.HTTP = server
	}

	// All components initialized successfully
	slog.Info("all components initialized successfully")
	return nil
}

func componentsClose() error {

	slog.Info("all components start closing")

	var errs []error
	// Close the HTTP server if it was initialized
	if runtime.HTTP != nil {
		err := runtime.HTTP.Close()
		if err != nil {
			errs = append(errs, err)
		}
		slog.Info("http service closed")
	}
	// Close the database if it was initialized
	if runtime.DB != nil {
		err := gormdb.Close(runtime.DB)
		if err != nil {
			errs = append(errs, err)
		}
		slog.Info("DB closed")
	}

	if len(errs) > 0 {
		all_error := errors.Join(errs...)
		slog.Error("errors occurred while closing components: " + all_error.Error())
		return all_error
	} else {
		slog.Info("all components successfully closed")
		return nil
	}

}

// WaitAndClose waits for all routines to complete and then closes all initialized components.
// Local cache and easylog are closed after all routines have completed.
// Previous global slog is restored after all components have been closed.
func WaitAndClose() error {
	slog.Info("waiting for all routines to complete before closing components")
	//wait for all routines to complete
	defer func() {
		if runtime.LCache != nil {
			runtime.LCache.Close()
		}
		_ = easylog.Close()
	}()
	EasyRoutine.Wait()
	return componentsClose()
}
