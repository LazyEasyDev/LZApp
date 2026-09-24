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
)

const (
	maxLogRecordBytes = 16 << 20
	logReadChunkBytes = 64 << 10
)

var logFilePattern = regexp.MustCompile(`^(debug|info|warn|err)_(\d{8})_([0-9]+)\.jsonl$`)

var errLogRecordTooLong = errors.New("log record exceeds maximum size")

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
	reader := reverseLogReader{source: file, offset: info.Size()}
	var records []storedRecord
	firstLine := true
	for len(records) < limit {
		data, position, err := reader.line()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read log file %q at byte %d: %w", fileName, position, err)
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

type reverseLogReader struct {
	source io.ReaderAt
	offset int64
	buffer []byte
}

func (reader *reverseLogReader) line() ([]byte, int64, error) {
	var parts [][]byte
	var size int
	var position int64
	for {
		if len(reader.buffer) == 0 {
			if reader.offset == 0 {
				if size == 0 {
					return nil, 0, io.EOF
				}
				break
			}
			chunkSize := min(int64(logReadChunkBytes), reader.offset)
			reader.offset -= chunkSize
			reader.buffer = make([]byte, int(chunkSize))
			if _, err := reader.source.ReadAt(reader.buffer, reader.offset); err != nil {
				if errors.Is(err, io.EOF) {
					err = io.ErrUnexpectedEOF
				}
				return nil, reader.offset, err
			}
		}

		newline := bytes.LastIndexByte(reader.buffer, '\n')
		part := reader.buffer[newline+1:]
		position = reader.offset + int64(newline+1)
		size += len(part)
		if size > maxLogRecordBytes+1 {
			return nil, position, errLogRecordTooLong
		}
		parts = append(parts, part)
		if newline >= 0 {
			reader.buffer = reader.buffer[:newline]
			break
		}
		reader.buffer = nil
	}

	data := make([]byte, size)
	for _, part := range parts {
		size -= len(part)
		copy(data[size:], part)
	}
	data = bytes.TrimSuffix(data, []byte{'\r'})
	if len(data) > maxLogRecordBytes {
		return nil, position, errLogRecordTooLong
	}
	return data, position, nil
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
