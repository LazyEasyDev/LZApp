package components

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/LazyEasyDev/EasyRoutine"

	"github.com/LazyEasyDev/LZApp/components/dbkv"
	"github.com/LazyEasyDev/LZApp/components/easylog"
	"github.com/LazyEasyDev/LZApp/components/easyroutine"
	"github.com/LazyEasyDev/LZApp/components/gormdb"
	"github.com/LazyEasyDev/LZApp/components/httpserver"
	"github.com/LazyEasyDev/LZApp/components/lcache"
	"github.com/LazyEasyDev/LZApp/components/security"
	"github.com/LazyEasyDev/LZApp/config"
	"gorm.io/gorm"
)

type Runtime struct {
	DB       *gorm.DB
	HTTP     *httpserver.Server
	Security *security.HMACTokenSigner
}

var runtime Runtime

func GetComponents() *Runtime {
	return &runtime
}

func GetDB() *gorm.DB {
	return runtime.DB
}

/*
	Init can never be called more than once or by multiple goroutines simultaneously.
	Any Error return will be followed by Close being called.
	Any Error will result in system termination.
*/

func InitDB(ctx context.Context, appConfig *config.AppConfig) error {
	slog.Info("initialize database...")
	db, err := gormdb.Init(ctx, appConfig)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	runtime.DB = db
	slog.Info("database initialized")
	return nil
}

func CloseDB() error {
	slog.Info("closing database...")
	if runtime.DB != nil {
		err := gormdb.Close(runtime.DB)
		if err != nil {
			slog.Error("failed to close database:" + err.Error())
			return fmt.Errorf("close database: %w", err)
		}
		slog.Info("database closed")
	}
	return nil
}

func InitHttpServer(appConfig *config.AppConfig) (*httpserver.Server, error) {
	slog.Info("initialize httpserver...")
	return httpserver.Init(appConfig.HTTP)
}

func CloseHttpServer() error {
	slog.Info("closing HTTP server...")
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
	if appConfig == nil {
		return fmt.Errorf("application configuration is required")
	}

	// Initialize the logging system
	if err := easylog.Init(appConfig.Log); err != nil {
		slog.Error("failed to initialize logging:" + err.Error())
		return fmt.Errorf("initialize logging: %w", err)
	}

	// Initialize the security component
	if appConfig.Security == nil {
		return fmt.Errorf("initialize security: security configuration is required")
	}
	signer, err := security.NewHMACTokenSigner([]byte(appConfig.Security.HMACKey), security.HMACTokenOptions{
		PayloadBytes:   appConfig.Security.HMACTokenBytes,
		SignatureBytes: appConfig.Security.HMACTokenBytes,
	})
	if err != nil {
		slog.Error("failed to initialize security:" + err.Error())
		return fmt.Errorf("initialize security: %w", err)
	}
	runtime.Security = signer
	if strings.TrimSpace(appConfig.Security.HMACKey) == "" {
		slog.Warn("HMAC key is empty; using a temporary signing key, tokens will be invalid after restart")
	}

	slog.Info("----------initialize components..................-------------")
	// Initialize the local cache
	lcache.Init(appConfig.Cache)

	if err := InitDB(ctx, appConfig); err != nil {
		return err
	} else {
		// Initialize the routine coordinator if enabled
		if runtime.DB != nil {
			// initialize the routine coordinator
			slog.Info("initialize easyroutine ...")
			if err := easyroutine.Init(ctx, runtime.DB); err != nil {
				slog.Error("failed to initialize routine coordinator:" + err.Error())
				return fmt.Errorf("initialize routine coordinator: %w", err)
			}
			slog.Info("easyroutine initialized")

			// Initialize the dbkv component if enabled
			slog.Info("initialize dbkv ...")
			if err := dbkv.Init(ctx, runtime.DB); err != nil {
				slog.Error("failed to initialize dbkv:" + err.Error())
				return fmt.Errorf("initialize dbkv: %w", err)
			}
			slog.Info("dbkv initialized")
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
	slog.Info("----------all components initialized successfully-------------")
	return nil
}

func closeComponents() error {
	fmt.Fprintln(os.Stderr) //ctr+c signal for newline
	slog.Info("----------closing components..................----------------")

	var errs []error

	// Close the HTTP server if it was initialized
	if err := CloseHttpServer(); err != nil {
		errs = append(errs, err)
	}

	// Close the dbkv component if it was initialized
	slog.Info("closing dbkv...")
	dbkv.Close()
	slog.Info("dbkv closed")

	// Close the database if it was initialized
	if err := CloseDB(); err != nil {
		errs = append(errs, err)
	}

	// Return any errors that occurred during the closing of components
	if len(errs) > 0 {
		combinedErr := errors.Join(errs...)
		slog.Error("errors occurred while closing components: " + combinedErr.Error())
		return combinedErr
	} else {
		slog.Info("----------all components successfully closed------------------")
		return nil
	}

}

// WaitAndClose waits for all routines to complete and then closes all initialized components.
// Local cache and easylog are closed after all routines have completed.
// Previous global slog is restored after all components have been closed.
func WaitAndClose() (closeErr error) {
	slog.Info("waiting for all routines to complete before closing components")
	slog.Info("--------------------------------------------------------------")
	//wait for all routines to complete
	defer func() {
		// Close the local cache component first
		lcache.Close()
		// Close the local cache component
		if err := easylog.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close logging: %w", err))
		}
	}()

	EasyRoutine.Wait()
	return closeComponents()
}
