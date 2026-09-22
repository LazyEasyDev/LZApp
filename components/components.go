package components

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	easyroutinelib "github.com/LazyEasyDev/EasyRoutine"
	cachelib "github.com/LazyEasyDev/LCache"

	"github.com/LazyEasyDev/LZApp/components/easylog"
	"github.com/LazyEasyDev/LZApp/components/easyroutine"
	"github.com/LazyEasyDev/LZApp/components/gormdb"
	"github.com/LazyEasyDev/LZApp/components/httpserver"
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

func InitDB(ctx context.Context, appConfig *config.AppConfig) (*gorm.DB, error) {
	if !appConfig.DB.Enabled {
		return nil, nil
	}
	return gormdb.Init(ctx, appConfig)
}

func CloseDB() error {
	if runtime.DB != nil {
		err := gormdb.Close(runtime.DB)
		if err != nil {
			slog.Error("failed to close database:" + err.Error())
			return fmt.Errorf("close database: %w", err)
		}
		slog.Info("DB closed")
	}
	return nil
}

func InitHttpServer(appConfig *config.AppConfig) (*httpserver.Server, error) {
	if !appConfig.HTTP.Enabled {
		return nil, nil
	}
	return httpserver.Init(appConfig.HTTP)
}

func CloseHttpServer() error {
	if runtime.HTTP != nil {
		err := runtime.HTTP.Close()
		if err != nil {
			slog.Error("failed to close HTTP server:" + err.Error())
			return fmt.Errorf("close HTTP server: %w", err)
		}
		slog.Info("HTTP server closed")
	}
	return nil
}

func Init(ctx context.Context, appConfig *config.AppConfig) error {
	// Initialize the logging system
	if err := easylog.Init(appConfig.Log); err != nil {
		slog.Error("failed to initialize logging:" + err.Error())
		return fmt.Errorf("initialize logging: %w", err)
	}
	// Initialize the local cache
	runtime.LCache = lcache.Init(appConfig.Cache)

	if db, err := InitDB(ctx, appConfig); err != nil {
		return fmt.Errorf("initialize database: %w", err)
	} else {
		runtime.DB = db
		// Initialize the routine coordinator if enabled
		if appConfig.EasyRoutine.Enabled {
			if err := easyroutine.Init(ctx, runtime.DB); err != nil {
				slog.Error("failed to initialize routine coordinator:" + err.Error())
				return fmt.Errorf("initialize routine coordinator: %w", err)
			}
		}
	}
	// Initialize the HTTP server if enabled
	if server, err := InitHttpServer(appConfig); err != nil {
		return fmt.Errorf("initialize HTTP server: %w", err)
	} else {
		runtime.HTTP = server
		slog.Info("Http server initialized", "https_port", appConfig.HTTP.HTTPSPort)
	}

	// All components initialized successfully
	slog.Info("all components initialized successfully")
	return nil
}

func closeComponents() error {

	slog.Info("all components start closing")

	var errs []error

	// Close the HTTP server if it was initialized
	if err := CloseHttpServer(); err != nil {
		errs = append(errs, err)
	}

	// Close the database if it was initialized
	if err := CloseDB(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		combinedErr := errors.Join(errs...)
		slog.Error("errors occurred while closing components: " + combinedErr.Error())
		return combinedErr
	} else {
		slog.Info("all components successfully closed")
		return nil
	}

}

// WaitAndClose waits for all routines to complete and then closes all initialized components.
// Local cache and easylog are closed after all routines have completed.
// Previous global slog is restored after all components have been closed.
func WaitAndClose() (closeErr error) {
	slog.Info("waiting for all routines to complete before closing components")
	//wait for all routines to complete
	defer func() {
		if runtime.LCache != nil {
			runtime.LCache.Close()
		}
		if err := easylog.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close logging: %w", err))
		}
	}()
	easyroutinelib.Wait()
	return closeComponents()
}
