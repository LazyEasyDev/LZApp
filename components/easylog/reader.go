package easylog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/icza/backscanner"
)

const (
	maxLogRecordBytes = 16 << 20
	logReadChunkBytes = 64 << 10
)

var logFilePattern = regexp.MustCompile(`^(debug|info|warn|err)_(\d{8})_([0-9]+)\.jsonl$`)

type ReadOptions struct {
	Directory         string
	DirectoryRelative string
	Level             string
	Tail              int
}

type storedRecord struct {
	time time.Time
	data []byte
}

type logFile struct {
	name     string
	level    string
	date     string
	sequence uint64
}

func Show(writer io.Writer, options ReadOptions) error {
	if writer == nil {
		return fmt.Errorf("log writer is required")
	}
	if options.Tail < 1 {
		return fmt.Errorf("tail must be positive")
	}

	directory, err := ResolveDirectory(options.Directory, options.DirectoryRelative)
	if err != nil {
		return err
	}
	prefix, err := levelPrefix(options.Level)
	if err != nil {
		return err
	}

	records, err := readRecords(filepath.Join(directory, "logs"), prefix, options.Tail)
	if err != nil {
		return err
	}
	sort.SliceStable(records, func(left, right int) bool {
		return records[left].time.After(records[right].time)
	})
	if len(records) > options.Tail {
		records = records[:options.Tail]
	}
	for index := len(records) - 1; index >= 0; index-- {
		if _, err := writer.Write(append(records[index].data, '\n')); err != nil {
			return fmt.Errorf("write log record: %w", err)
		}
	}
	return nil
}

func readRecords(directory, selectedPrefix string, limit int) ([]storedRecord, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read log directory: %w", err)
	}

	var files []logFile
	for _, entry := range entries {
		match := logFilePattern.FindStringSubmatch(entry.Name())
		if entry.IsDir() || match == nil || (selectedPrefix != "" && match[1] != selectedPrefix) {
			continue
		}
		sequence, err := strconv.ParseUint(match[3], 10, 64)
		if err != nil {
			continue
		}
		files = append(files, logFile{name: entry.Name(), level: match[1], date: match[2], sequence: sequence})
	}
	sort.Slice(files, func(left, right int) bool {
		if files[left].date != files[right].date {
			return files[left].date > files[right].date
		}
		if files[left].sequence != files[right].sequence {
			return files[left].sequence > files[right].sequence
		}
		return files[left].name > files[right].name
	})

	var records []storedRecord
	counts := make(map[string]int)
	for _, file := range files {
		remaining := limit - counts[file.level]
		if remaining == 0 {
			continue
		}
		tail, err := readLogFile(filepath.Join(directory, file.name), remaining)
		if err != nil {
			return nil, err
		}
		records = append(records, tail...)
		counts[file.level] += len(tail)
	}
	return records, nil
}

func readLogFile(filePath string, limit int) ([]storedRecord, error) {
	fileName := filepath.Base(filePath)
	file, err := os.Open(filePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", fileName, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat log file %q: %w", fileName, err)
	}
	scanner := backscanner.NewOptions(file, int(info.Size()), &backscanner.Options{
		ChunkSize:     logReadChunkBytes,
		MaxBufferSize: maxLogRecordBytes + logReadChunkBytes + 2,
	})
	var records []storedRecord
	firstLine := true
	for len(records) < limit {
		data, position, err := scanner.LineBytes()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read log file %q: %w", fileName, err)
		}
		if len(data) > maxLogRecordBytes {
			return nil, fmt.Errorf("read log file %q at byte %d: %w", fileName, position, backscanner.ErrLongLine)
		}
		trailingFragment := firstLine
		firstLine = false
		data = bytes.TrimSpace(data)
		if len(data) == 0 {
			continue
		}
		var metadata struct {
			Time time.Time `json:"time"`
		}
		if err := json.Unmarshal(data, &metadata); err != nil {
			if trailingFragment {
				var fragment json.RawMessage
				if errors.Is(json.NewDecoder(bytes.NewReader(data)).Decode(&fragment), io.ErrUnexpectedEOF) {
					continue
				}
			}
			return nil, fmt.Errorf("decode %s at byte %d: %w", fileName, position, err)
		}
		if metadata.Time.IsZero() {
			return nil, fmt.Errorf("decode %s at byte %d: missing time", fileName, position)
		}
		records = append(records, storedRecord{time: metadata.Time, data: bytes.Clone(data)})
	}
	return records, nil
}

func levelPrefix(level string) (string, error) {
	if strings.TrimSpace(level) == "" {
		return "", nil
	}
	parsed, err := parseLevel(level)
	if err != nil {
		return "", err
	}
	if parsed == slog.LevelError {
		return "err", nil
	}
	return strings.ToLower(parsed.String()), nil
}
