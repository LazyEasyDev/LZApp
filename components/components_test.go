package components

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	easyloglib "github.com/LazyEasyDev/EasyLog"
	"gorm.io/gorm"
)

func TestWaitAndClosePreservesCleanupErrors(t *testing.T) {
	loggerErr := errors.New("log file sync failed")
	for _, testCase := range []struct {
		name      string
		loggerErr error
		invalidDB bool
	}{
		{name: "successful cleanup"},
		{name: "logger failure", loggerErr: loggerErr},
		{name: "database failure", invalidDB: true},
		{name: "both failures", loggerErr: loggerErr, invalidDB: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			previousRuntime := runtime
			runtime = Runtime{}
			t.Cleanup(func() { runtime = previousRuntime })
			if testCase.invalidDB {
				runtime.DB = &gorm.DB{Config: &gorm.Config{}}
			}
			previousLogger := slog.Default()
			output := &cleanupLogOutput{closeErr: testCase.loggerErr}
			if err := easyloglib.InitWithOutputs(easyloglib.Options{}, []easyloglib.Output{output}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = easyloglib.Close() })

			err := WaitAndClose()
			if testCase.loggerErr == nil && !testCase.invalidDB && err != nil {
				t.Fatalf("successful cleanup returned %v", err)
			}
			if testCase.loggerErr != nil {
				if !errors.Is(err, testCase.loggerErr) {
					t.Fatalf("cleanup error = %v, want logger failure", err)
				}
				if !strings.Contains(err.Error(), "close logging:") {
					t.Errorf("logger failure lacks cleanup context: %v", err)
				}
			}
			if testCase.invalidDB && !errors.Is(err, gorm.ErrInvalidDB) {
				t.Errorf("cleanup error = %v, want database failure", err)
			}
			if output.closeCalls != 1 {
				t.Errorf("logger closed %d times, want 1", output.closeCalls)
			}
			if slog.Default() != previousLogger {
				t.Error("cleanup did not restore the previous logger")
			}
		})
	}
}

type cleanupLogOutput struct {
	closeErr   error
	closeCalls int
}

func (*cleanupLogOutput) WriteRecord(slog.Record, []byte) error { return nil }

func (*cleanupLogOutput) Sync() error { return nil }

func (output *cleanupLogOutput) Close() error {
	output.closeCalls++
	return output.closeErr
}
