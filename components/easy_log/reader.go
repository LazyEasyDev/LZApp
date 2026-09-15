package easylog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const maxLogRecordBytes = 16 << 20

var logFilePattern = regexp.MustCompile(`^(debug|info|warn|err)_\d{8}_[0-9]+\.jsonl$`)

type ReadOptions struct {
	Directory string
	Level     string
	Tail      int
}

type storedRecord struct {
	time     time.Time
	fileName string
	line     int
	data     []byte
}

func Show(writer io.Writer, options ReadOptions) error {
	if writer == nil {
		return fmt.Errorf("log writer is required")
	}
	if options.Tail < 1 {
		return fmt.Errorf("tail must be positive")
	}

	directory, err := ResolveDirectory(options.Directory)
	if err != nil {
		return err
	}
	prefix, err := levelPrefix(options.Level)
	if err != nil {
		return err
	}

	records, err := readRecords(filepath.Join(directory, "logs"), prefix)
	if err != nil {
		return err
	}
	sort.SliceStable(records, func(left, right int) bool {
		if !records[left].time.Equal(records[right].time) {
			return records[left].time.Before(records[right].time)
		}
		if records[left].fileName != records[right].fileName {
			return records[left].fileName < records[right].fileName
		}
		return records[left].line < records[right].line
	})

	if len(records) > options.Tail {
		records = records[len(records)-options.Tail:]
	}
	for _, record := range records {
		if _, err := writer.Write(append(record.data, '\n')); err != nil {
			return fmt.Errorf("write log record: %w", err)
		}
	}
	return nil
}

func readRecords(directory, selectedPrefix string) ([]storedRecord, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return []storedRecord{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read log directory: %w", err)
	}

	var records []storedRecord
	for _, entry := range entries {
		if entry.IsDir() || !logFilePattern.MatchString(entry.Name()) {
			continue
		}
		if selectedPrefix != "" && !strings.HasPrefix(entry.Name(), selectedPrefix+"_") {
			continue
		}
		fileRecords, err := readLogFile(filepath.Join(directory, entry.Name()), entry.Name())
		if err != nil {
			return nil, err
		}
		records = append(records, fileRecords...)
	}
	return records, nil
}

func readLogFile(filePath, fileName string) ([]storedRecord, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", fileName, err)
	}
	defer file.Close()

	var records []storedRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxLogRecordBytes)
	for line := 1; scanner.Scan(); line++ {
		data := bytes.TrimSpace(scanner.Bytes())
		if len(data) == 0 {
			continue
		}
		var metadata struct {
			Time time.Time `json:"time"`
		}
		if err := json.Unmarshal(data, &metadata); err != nil {
			return nil, fmt.Errorf("decode %s line %d: %w", fileName, line, err)
		}
		if metadata.Time.IsZero() {
			return nil, fmt.Errorf("decode %s line %d: missing time", fileName, line)
		}
		records = append(records, storedRecord{
			time:     metadata.Time,
			fileName: fileName,
			line:     line,
			data:     bytes.Clone(data),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read log file %q: %w", fileName, err)
	}
	return records, nil
}

func levelPrefix(level string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "":
		return "", nil
	case "debug":
		return "debug", nil
	case "info":
		return "info", nil
	case "warn", "warning":
		return "warn", nil
	case "err", "error":
		return "err", nil
	default:
		return "", fmt.Errorf("unknown log level %q", level)
	}
}
