package config

type Profile string

const (
	ProfileDebug   Profile = "debug"
	ProfileRelease Profile = "release"
)

type LogConfig struct {
	Level       string `json:"level"`
	Directory   string `json:"directory"`
	AddSource   bool   `json:"add_source"`
	ToTerminal  bool   `json:"to_terminal"`
	ShowLogTail uint   `json:"show_log_tail"`
}

type LocalCacheConfig struct {
	MaxTTLSeconds int64 `json:"max_ttl_seconds"`
}

////below are optional///////

type DBConfig struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"db_name"`
	Charset  string `json:"charset"`
}

type HTTPConfig struct {
	Enabled          bool   `json:"enabled"`
	HTTPSPort        int    `json:"https_port"`
	HTTPSCertificate string `json:"https_certificate"`
	HTTPSKey         string `json:"https_key"`
}

type EasyRoutineConfig struct {
	Enabled bool `json:"enabled"`
}

type AppConfig struct {
	Profile     Profile            `json:"profile"`
	HTTP        *HTTPConfig        `json:"http"`
	DB          *DBConfig          `json:"database"`
	Log         *LogConfig         `json:"log"`
	Cache       *LocalCacheConfig  `json:"cache"`
	EasyRoutine *EasyRoutineConfig `json:"easy_routine"`
}

var default_AppConfig = AppConfig{
	Profile: ProfileRelease,
	Log: &LogConfig{
		Directory:   "logs",
		ToTerminal:  true,
		ShowLogTail: 10,
	},
	Cache: &LocalCacheConfig{
		MaxTTLSeconds: 24 * 60 * 60,
	},
	////optional
	HTTP: &HTTPConfig{
		Enabled:          false,
		HTTPSPort:        8443,
		HTTPSCertificate: PEM_STR,
		HTTPSKey:         KEY_STR,
	},
	DB: &DBConfig{
		Enabled: false,
		Host:    "127.0.0.1",
		Port:    3306,
	},
	EasyRoutine: &EasyRoutineConfig{
		Enabled: true,
	},
}

var currentAppConfig *AppConfig

var config_initialized bool

func InitConfig(profile Profile) {
	if config_initialized {
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

	config_initialized = true
}

func GetConfig() *AppConfig {
	return currentAppConfig
}
