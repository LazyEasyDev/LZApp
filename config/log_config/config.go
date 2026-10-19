package log_config

type LogConfig struct {
	Level string `json:"level"`
	// BaseDirectory contains the logs/ subdirectory created by EasyLog.
	BaseDirectory         string `json:"base_directory"`
	BaseDirectoryRelative string `json:"base_directory_relative"` // "app" or "cwd"; empty means "cwd"
	AddSource             bool   `json:"add_source"`
	ToTerminal            bool   `json:"to_terminal"`
	ShowLogTail           uint   `json:"show_log_tail"`
}
