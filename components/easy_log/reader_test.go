package easylog

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShowMergesLevelsThenTailsInAscendingTime(t *testing.T) {
	directory := t.TempDir()
	logsDirectory := filepath.Join(directory, "logs")
	if err := os.MkdirAll(logsDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLogFile(t, logsDirectory, "info_20260915_0.jsonl", []string{
		`{"time":"2026-09-15T10:00:00Z","level":"INFO","msg":"first"}`,
		`{"time":"2026-09-15T10:02:00Z","level":"INFO","msg":"third"}`,
	})
	writeLogFile(t, logsDirectory, "err_20260915_0.jsonl", []string{
		`{"time":"2026-09-15T10:01:00Z","level":"ERROR","msg":"second"}`,
	})

	var output bytes.Buffer
	if err := Show(&output, ReadOptions{Directory: directory, Tail: 2}); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); !strings.Contains(got, `"msg":"second"`) || !strings.Contains(got, `"msg":"third"`) || strings.Contains(got, `"msg":"first"`) {
		t.Fatalf("unexpected merged tail:\n%s", got)
	}
	if strings.Index(output.String(), `"msg":"second"`) > strings.Index(output.String(), `"msg":"third"`) {
		t.Fatalf("records are not ascending:\n%s", output.String())
	}
}

func TestShowFiltersAliases(t *testing.T) {
	directory := t.TempDir()
	logsDirectory := filepath.Join(directory, "logs")
	if err := os.MkdirAll(logsDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLogFile(t, logsDirectory, "debug_20260915_0.jsonl", []string{
		`{"time":"2026-09-15T10:00:00Z","level":"DEBUG","msg":"debug"}`,
	})
	writeLogFile(t, logsDirectory, "err_20260915_0.jsonl", []string{
		`{"time":"2026-09-15T10:01:00Z","level":"ERROR","msg":"error"}`,
	})

	var output bytes.Buffer
	if err := Show(&output, ReadOptions{Directory: directory, Level: "error", Tail: 10}); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); !strings.Contains(got, `"msg":"error"`) || strings.Contains(got, `"msg":"debug"`) {
		t.Fatalf("unexpected filtered logs:\n%s", got)
	}
}

func writeLogFile(t *testing.T, directory, name string, records []string) {
	t.Helper()
	data := []byte(strings.Join(records, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(directory, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
