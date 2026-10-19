package log_config

type LogConfig struct {
	Level             string `json:"level"`
	Directory         string `json:"directory"`
	DirectoryRelative string `json:"directory_relative"` // "app" or "cwd" , empty means "cwd"
	AddSource         bool   `json:"add_source"`
	ToTerminal        bool   `json:"to_terminal"`
	ShowLogTail       uint   `json:"show_log_tail"`
}
