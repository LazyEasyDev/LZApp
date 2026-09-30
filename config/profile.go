package config

import (
	"github.com/LazyEasyDev/LZApp/config/db_config"
	"github.com/LazyEasyDev/LZApp/config/email_config"
	"github.com/LazyEasyDev/LZApp/config/http_config"
	"github.com/LazyEasyDev/LZApp/config/lcache_config"
	"github.com/LazyEasyDev/LZApp/config/log_config"
	"github.com/LazyEasyDev/LZApp/config/redis_config"
	"github.com/LazyEasyDev/LZApp/config/security_config"
)

type Profile string

const (
	ProfileDebug   Profile = "debug"
	ProfileRelease Profile = "release"
)

////below are optional///////

type AppConfig struct {
	Profile        Profile                               `json:"profile"`
	HTTP           *http_config.HTTPConfig               `json:"http"`
	DB             *db_config.DBConfig                   `json:"database"`
	Email          *email_config.EmailConfig             `json:"email"`
	Redis          *redis_config.RedisConfig             `json:"redis"`
	Log            *log_config.LogConfig                 `json:"log"`
	Cache          *lcache_config.LocalCacheConfig       `json:"cache"`
	SecurityHMAC   *security_config.SecurityHMACConfig   `json:"security_hmac"`
	SecurityBcrypt *security_config.SecurityBcryptConfig `json:"security_bcrypt"`
}

func newDefaultConfig() AppConfig {
	return AppConfig{
		Profile: ProfileRelease,
		Log: &log_config.LogConfig{
			Directory:         "logs",
			DirectoryRelative: "app",
			ToTerminal:        true,
			ShowLogTail:       10,
			AddSource:         false,
			Level:             "info",
		},
		Cache: &lcache_config.LocalCacheConfig{
			MaxTTLSeconds: 24 * 60 * 60,
		},
		SecurityHMAC: &security_config.SecurityHMACConfig{
			HMACKey:        "",
			HMACTokenBytes: 16,
		},
		SecurityBcrypt: &security_config.SecurityBcryptConfig{
			Cost: 10,
		},
		////optional
		HTTP: &http_config.HTTPConfig{
			APITokenCookieName:       "api_token",
			HTTPSPort:                443,
			HTTPSCertificate:         http_config.HTTPSCertificatePEM,
			HTTPSKey:                 http_config.HTTPSPrivateKeyPEM,
			ReadHeaderTimeoutSeconds: 10,
			IdleTimeoutSeconds:       60,
			ReadTimeoutSeconds:       0,
			WriteTimeoutSeconds:      0,
			ShutdownTimeoutSeconds:   60,
		},
		DB: &db_config.DBConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			LogLevel: "silent",
			Charset:  "utf8mb4",
		},
		Email: &email_config.EmailConfig{
			Port:           587,
			TimeoutSeconds: 8,
		},
		Redis: &redis_config.RedisConfig{
			ClusterMode:         true,
			TLS:                 true,
			InsecureSkipVerify:  true,
			DialTimeoutSeconds:  6,
			ReadTimeoutSeconds:  5,
			WriteTimeoutSeconds: 5,
		},
	}
}

var currentAppConfig *AppConfig

var configInitialized bool

func InitConfig(profile Profile) {
	if configInitialized {
		return
	}

	switch profile {
	case ProfileDebug:
		currentAppConfig = &debugAppConfig
	case ProfileRelease:
		currentAppConfig = &releaseAppConfig
	default:
		return
	}

	configInitialized = true
}

func GetConfig() *AppConfig {
	if !configInitialized {
		InitConfig(ProfileRelease)
	}
	return currentAppConfig
}
