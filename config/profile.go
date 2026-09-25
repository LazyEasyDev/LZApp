package config

type Profile string

const (
	ProfileDebug   Profile = "debug"
	ProfileRelease Profile = "release"
)

type LogConfig struct {
	Level             string `json:"level"`
	Directory         string `json:"directory"`
	DirectoryRelative string `json:"directory_relative"`
	AddSource         bool   `json:"add_source"`
	ToTerminal        bool   `json:"to_terminal"`
	ShowLogTail       uint   `json:"show_log_tail"`
}

type LocalCacheConfig struct {
	MaxTTLSeconds int64 `json:"max_ttl_seconds"`
}

type SecurityConfig struct {
	HMACKey        string `json:"-"`
	HMACTokenBytes int    `json:"hmac_token_bytes"` // 16|24|32
}

////below are optional///////

type DBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"db_name"`
	Charset  string `json:"charset"`
}

type HTTPConfig struct {
	APITokenCookieName       string `json:"api_token_cookie_name"`
	HTTPSPort                int    `json:"https_port"`
	HTTPSCertificate         string `json:"https_certificate"`
	HTTPSKey                 string `json:"https_key"`
	ReadHeaderTimeoutSeconds int    `json:"read_header_timeout_seconds"`
	IdleTimeoutSeconds       int    `json:"idle_timeout_seconds"`
	ReadTimeoutSeconds       int    `json:"read_timeout_seconds"`
	WriteTimeoutSeconds      int    `json:"write_timeout_seconds"`
	ShutdownTimeoutSeconds   int    `json:"shutdown_timeout_seconds"`
}

type AppConfig struct {
	Profile  Profile           `json:"profile"`
	HTTP     *HTTPConfig       `json:"http"`
	DB       *DBConfig         `json:"database"`
	Log      *LogConfig        `json:"log"`
	Cache    *LocalCacheConfig `json:"cache"`
	Security *SecurityConfig   `json:"security"`
}

func newDefaultConfig() AppConfig {
	return AppConfig{
		Profile: ProfileRelease,
		Log: &LogConfig{
			Directory:         "logs",
			DirectoryRelative: "app",
			ToTerminal:        true,
			ShowLogTail:       10,
		},
		Cache: &LocalCacheConfig{
			MaxTTLSeconds: 24 * 60 * 60,
		},
		Security: &SecurityConfig{
			HMACKey:        "",
			HMACTokenBytes: 32,
		},
		////optional
		HTTP: &HTTPConfig{
			APITokenCookieName:       "api_token",
			HTTPSPort:                8443,
			HTTPSCertificate:         HTTPSCertificatePEM,
			HTTPSKey:                 HTTPSPrivateKeyPEM,
			ReadHeaderTimeoutSeconds: 10,
			IdleTimeoutSeconds:       60,
			ReadTimeoutSeconds:       0,
			WriteTimeoutSeconds:      0,
			ShutdownTimeoutSeconds:   60,
		},
		DB: &DBConfig{
			Host: "127.0.0.1",
			Port: 3306,
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
