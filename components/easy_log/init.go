package easylog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	easyloglib "github.com/LazyEasyDev/EasyLog"
	"github.com/LazyEasyDev/LZApp/config"
)

func Init(logConfig *config.LogConfig) error {
	level, err := parseLevel(logConfig.Level)
	if err != nil {
		return err
	}
	directory, err := ResolveDirectory(logConfig.Directory)
	if err != nil {
		return err
	}

	var terminal io.Writer
	if logConfig.ToTerminal {
		terminal = os.Stderr
	}
	return easyloglib.Init(
		easyloglib.Options{
			Level:     level,
			AddSource: logConfig.AddSource,
		},
		&easyloglib.FileOptions{Directory: directory},
		terminal,
	)
}

func Close() error {
	return easyloglib.Close()
}

func ResolveDirectory(directory string) (string, error) {
	if directory == "" {
		return "", fmt.Errorf("log directory is required")
	}
	if filepath.IsAbs(directory) {
		return filepath.Clean(directory), nil
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
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "err", "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q", value)
	}
}
