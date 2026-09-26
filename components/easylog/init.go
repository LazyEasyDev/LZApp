package easylog

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/LazyEasyDev/EasyLog"
	"github.com/LazyEasyDev/LZApp/config/log_config"
)

func Init(logConfig *log_config.LogConfig) error {
	if logConfig == nil {
		return fmt.Errorf("log configuration is required")
	}

	level, err := parseLevel(logConfig.Level)
	if err != nil {
		return err
	}
	directory, err := ResolveDirectory(logConfig.Directory, logConfig.DirectoryRelative)
	if err != nil {
		return err
	}

	options := EasyLog.InitOptions{
		Runtime: EasyLog.Options{
			Level:     level,
			AddSource: logConfig.AddSource,
		},
		File: &EasyLog.FileOptions{BaseDirectory: directory},
	}
	if logConfig.ToTerminal {
		options.Terminal = &EasyLog.TerminalOptions{Writer: os.Stderr}
	}

	return EasyLog.Init(options)
}

func Close() error {
	return EasyLog.Close()
}

func ResolveDirectory(directory, relativeTo string) (string, error) {
	if directory == "" {
		return "", fmt.Errorf("log directory is required")
	}
	if relativeTo != "" && relativeTo != "cwd" && relativeTo != "app" {
		return "", fmt.Errorf("log directory_relative must be app or cwd, got %q", relativeTo)
	}
	if filepath.IsAbs(directory) {
		return filepath.Clean(directory), nil
	}
	if relativeTo == "app" {
		executable, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("resolve application directory: %w", err)
		}
		return filepath.Join(filepath.Dir(executable), directory), nil
	}
	resolved, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve log directory: %w", err)
	}
	return resolved, nil
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "err", "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q", value)
	}
}
