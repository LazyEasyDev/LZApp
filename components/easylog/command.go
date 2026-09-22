package easylog

import (
	"io"

	"github.com/LazyEasyDev/LZApp/config"
)

func ShowLogs(writer io.Writer, level string, tail int) error {
	logConfig := config.GetConfig().Log
	if tail == 0 {
		tail = int(logConfig.ShowLogTail)
	}
	return Show(writer, ReadOptions{
		Directory:         logConfig.Directory,
		DirectoryRelative: logConfig.DirectoryRelative,
		Level:             level,
		Tail:              tail,
	})
}
