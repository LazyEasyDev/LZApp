package config

import (
	"github.com/LazyEasyDev/LZApp/config/db_config"
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
	Profile  Profile                         `json:"profile"`
	HTTP     *http_config.HTTPConfig         `json:"http"`
	DB       *db_config.DBConfig             `json:"database"`
	Redis    *redis_config.RedisConfig       `json:"redis"`
	Log      *log_config.LogConfig           `json:"log"`
	Cache    *lcache_config.LocalCacheConfig `json:"cache"`
	Security *security_config.SecurityConfig `json:"security"`
}

func newDefaultConfig() AppConfig {
	return AppConfig{
		Profile: ProfileRelease,
		Log: &log_config.LogConfig{
			Directory:         "logs",
			DirectoryRelative: "app",
			ToTerminal:        true,
			ShowLogTail:       10,
		},
		Cache: &lcache_config.LocalCacheConfig{
			MaxTTLSeconds: 24 * 60 * 60,
		},
		Security: &security_config.SecurityConfig{
			HMACKey:        "",
			HMACTokenBytes: 32,
		},
		////optional
		HTTP: &http_config.HTTPConfig{
			APITokenCookieName:       "api_token",
			HTTPSPort:                8443,
			HTTPSCertificate:         http_config.HTTPSCertificatePEM,
			HTTPSKey:                 http_config.HTTPSPrivateKeyPEM,
			ReadHeaderTimeoutSeconds: 10,
			IdleTimeoutSeconds:       60,
			ReadTimeoutSeconds:       0,
			WriteTimeoutSeconds:      0,
			ShutdownTimeoutSeconds:   60,
		},
		DB: &db_config.DBConfig{
			Host: "127.0.0.1",
			Port: 3306,
		},
		Redis: &redis_config.RedisConfig{
			ClusterMode:         true,
			TLS:                 true,
			InsecureSkipVerify:  true,
			Addrs:               []string{"127.0.0.1:7001"},
			Username:            "lzapp",
			KeyPrefix:           "lzapp:",
			Password:            "b9b07f4ad4a526f0bda7794fbaa922fcec22cdf737718363c08e48f978bf2071",
			DialTimeoutSeconds:  5,
			ReadTimeoutSeconds:  3,
			WriteTimeoutSeconds: 3,
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
