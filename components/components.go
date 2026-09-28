package components

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/LazyEasyDev/EasyRoutine"

	"github.com/LazyEasyDev/LZApp/components/easylog"
	"github.com/LazyEasyDev/LZApp/components/easyroutine"
	"github.com/LazyEasyDev/LZApp/components/email"
	"github.com/LazyEasyDev/LZApp/components/gormdb"
	"github.com/LazyEasyDev/LZApp/components/httpserver"
	"github.com/LazyEasyDev/LZApp/components/lcache"
	"github.com/LazyEasyDev/LZApp/components/redis"
	"github.com/LazyEasyDev/LZApp/components/security"
	"github.com/LazyEasyDev/LZApp/config"
	"github.com/LazyEasyDev/LZApp/config/db_config"
	"github.com/LazyEasyDev/LZApp/config/email_config"
	"github.com/LazyEasyDev/LZApp/config/http_config"
	"github.com/LazyEasyDev/LZApp/config/redis_config"
	"github.com/LazyEasyDev/LZApp/config/security_config"
	"gorm.io/gorm"
)

type Runtime struct {
	DB             *gorm.DB
	Redis          *redis.Client
	HTTP           *httpserver.Server
	TokenSigner    *security.HMACTokenSigner
	PasswordHasher *security.BcryptHasher
	EmailSender    *email.Sender
}

var runtime Runtime

// func GetComponents() *Runtime {
// 	return &runtime
// }

func GetDB() *gorm.DB {
	return runtime.DB
}

func GetRedis() *redis.Client {
	return runtime.Redis
}

func GetHTTP() *httpserver.Server {
	return runtime.HTTP
}

func GetTokenSigner() *security.HMACTokenSigner {
	return runtime.TokenSigner
}

func GetPasswordHasher() *security.BcryptHasher {
	return runtime.PasswordHasher
}

func GetEmailSender() *email.Sender {
	return runtime.EmailSender
}

/*
	Init can never be called more than once or by multiple goroutines simultaneously.
	Any Error return will be followed by Close being called.
	Any Error will result in system termination.
*/

func InitDB(ctx context.Context, dbConfig *db_config.DBConfig) error {

	slog.Info("initialize database...")
	db, err := gormdb.New(ctx, dbConfig)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	runtime.DB = db
	slog.Info("database initialized")
	return nil
}

func InitEmail(emailConfig *email_config.EmailConfig) error {
	slog.Info("initialize email...")
	sender, err := email.New(emailConfig)
	if err != nil {
		return fmt.Errorf("initialize email: %w", err)
	}
	runtime.EmailSender = sender
	slog.Info("email initialized")
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

func InitRedis(ctx context.Context, redisConfig *redis_config.RedisConfig) error {
	slog.Info("initialize Redis...")
	client, err := redis.New(ctx, redisConfig)
	if err != nil {
		return fmt.Errorf("initialize Redis: %w", err)
	}
	runtime.Redis = client
	slog.Info("Redis initialized")
	return nil
}

func CloseRedis() error {
	if runtime.Redis == nil {
		return nil
	}
	slog.Info("closing Redis...")
	err := runtime.Redis.Close()
	runtime.Redis = nil
	if err != nil {
		return fmt.Errorf("close Redis: %w", err)
	}
	slog.Info("Redis closed")
	return nil
}

func InitSecurity(hmacConfig *security_config.SecurityHMACConfig, bcryptConfig *security_config.SecurityBcryptConfig) error {

	// Initialize the password hasher first.
	passwordHasher, err := security.NewBcryptHasher(bcryptConfig)
	if err != nil {
		return fmt.Errorf("initialize password hasher: %w", err)
	}

	// Initialize the token signer first.
	signer, err := security.NewHMACTokenSigner(hmacConfig)
	if err != nil {
		return fmt.Errorf("initialize security: %w", err)
	}
	runtime.TokenSigner = signer
	if strings.TrimSpace(hmacConfig.HMACKey) == "" {
		slog.Warn("HMAC key is empty; using a temporary signing key, tokens will be invalid after restart")
	}
	// Initialize the password hasher after the token signer.
	runtime.PasswordHasher = passwordHasher

	return nil
}

func InitHttpServer(httpConfig *http_config.HTTPConfig) error {
	slog.Info("initialize httpserver...")
	srv, err := httpserver.New(httpConfig)
	if err != nil {
		return fmt.Errorf("initialize HTTP server: %w", err)
	}
	runtime.HTTP = srv
	slog.Info("httpserver initialized")
	return nil
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

	slog.Info("----------initialize components..................-------------")
	// Check if the application configuration is provided
	if appConfig == nil {
		slog.Error("application configuration is required")
		return fmt.Errorf("application configuration is required")
	}
	// Initialize the logging system
	if err := easylog.Init(appConfig.Log); err != nil {
		slog.Error("failed to initialize logging:" + err.Error())
		return fmt.Errorf("initialize logging: %w", err)
	}
	// Initialize the security component
	if err := InitSecurity(appConfig.SecurityHMAC, appConfig.SecurityBcrypt); err != nil {
		slog.Error("failed to initialize security:" + err.Error())
		return fmt.Errorf("initialize security: %w", err)
	}

	// Initialize the local cache
	lcache.Init(appConfig.Cache)

	if err := InitRedis(ctx, appConfig.Redis); err != nil {
		return err
	}

	// Initialize the database component
	if err := InitDB(ctx, appConfig.DB); err != nil {
		slog.Error("failed to initialize database:" + err.Error())
		return fmt.Errorf("initialize database: %w", err)
	}

	sqlDB, err := gormdb.SQLDB(GetDB())
	if err != nil {
		slog.Error("failed to get SQL database:" + err.Error())
		return fmt.Errorf("get SQL database: %w", err)
	}

	// initialize the routine coordinator
	if err := easyroutine.Init(ctx, sqlDB); err != nil {
		slog.Error("failed to initialize routine coordinator:" + err.Error())
		return fmt.Errorf("initialize routine coordinator: %w", err)
	}

	// Initialize the HTTP server if enabled
	if err := InitHttpServer(appConfig.HTTP); err != nil {
		slog.Error("failed to initialize HTTP server:" + err.Error())
		return fmt.Errorf("initialize HTTP server: %w", err)
	}

	// Initialize the email component
	if err := InitEmail(appConfig.Email); err != nil {
		slog.Error("failed to initialize email:" + err.Error())
		return fmt.Errorf("initialize email: %w", err)
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
	if err := CloseRedis(); err != nil {
		errs = append(errs, err)
	}
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
func WaitAndClose() error {
	slog.Info("waiting for all routines to complete before closing components")
	slog.Info("--------------------------------------------------------------")
	//wait for all routines to complete
	EasyRoutine.Wait()
	return Close()
}

func Close() (closeErr error) {
	defer func() {
		// Close the local cache component first
		lcache.Close()
		// Close the local cache component
		if err := easylog.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close logging: %w", err))
		}
	}()

	return closeComponents()
}
