package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LazyEasyDev/LZApp/config"
)

func TestInitUsesConfiguredTimeouts(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		config config.HTTPConfig
	}{
		{
			name: "custom timeouts",
			config: config.HTTPConfig{
				HTTPSPort:                8443,
				ReadHeaderTimeoutSeconds: 2,
				IdleTimeoutSeconds:       3,
				ReadTimeoutSeconds:       4,
				WriteTimeoutSeconds:      5,
				ShutdownTimeoutSeconds:   6,
			},
		},
		{
			name: "zero server timeouts",
			config: config.HTTPConfig{
				HTTPSPort:              8443,
				ShutdownTimeoutSeconds: 1,
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			srv, err := Init(&testCase.config)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = srv.Close() })
			for _, timeout := range []struct {
				name    string
				got     time.Duration
				seconds int
			}{
				{"read header", srv.server.ReadHeaderTimeout, testCase.config.ReadHeaderTimeoutSeconds},
				{"idle", srv.server.IdleTimeout, testCase.config.IdleTimeoutSeconds},
				{"read", srv.server.ReadTimeout, testCase.config.ReadTimeoutSeconds},
				{"write", srv.server.WriteTimeout, testCase.config.WriteTimeoutSeconds},
				{"shutdown", srv.shutdownTimeout, testCase.config.ShutdownTimeoutSeconds},
			} {
				if want := time.Duration(timeout.seconds) * time.Second; timeout.got != want {
					t.Errorf("%s timeout = %v, want %v", timeout.name, timeout.got, want)
				}
			}
		})
	}
}

func TestInitRejectsInvalidTimeouts(t *testing.T) {
	for _, field := range []struct {
		name string
		set  func(*config.HTTPConfig, int)
	}{
		{"read_header_timeout_seconds", func(cfg *config.HTTPConfig, value int) { cfg.ReadHeaderTimeoutSeconds = value }},
		{"idle_timeout_seconds", func(cfg *config.HTTPConfig, value int) { cfg.IdleTimeoutSeconds = value }},
		{"read_timeout_seconds", func(cfg *config.HTTPConfig, value int) { cfg.ReadTimeoutSeconds = value }},
		{"write_timeout_seconds", func(cfg *config.HTTPConfig, value int) { cfg.WriteTimeoutSeconds = value }},
		{"shutdown_timeout_seconds", func(cfg *config.HTTPConfig, value int) { cfg.ShutdownTimeoutSeconds = value }},
	} {
		invalidValues := []int{-1}
		if strconv.IntSize == 64 {
			invalidValues = append(invalidValues, int(^uint(0)>>1))
		}
		if field.name == "shutdown_timeout_seconds" {
			invalidValues = append(invalidValues, 0)
		}
		for _, value := range invalidValues {
			t.Run(field.name+"/"+strconv.Itoa(value), func(t *testing.T) {
				cfg := &config.HTTPConfig{HTTPSPort: 8443, ShutdownTimeoutSeconds: 60}
				field.set(cfg, value)
				srv, err := Init(cfg)
				if err == nil || !strings.Contains(err.Error(), field.name) {
					t.Fatalf("Init error = %v, want error naming %s", err, field.name)
				}
				if srv != nil {
					t.Error("invalid configuration returned a server")
				}
			})
		}
	}
}

func TestShutdownUsesConfiguredDeadline(t *testing.T) {
	srv, err := Init(&config.HTTPConfig{HTTPSPort: 8443, ShutdownTimeoutSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	srv.server.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	})
	testServer := httptest.NewUnstartedServer(srv.server.Handler)
	testServer.Config = srv.server
	testServer.Start()
	t.Cleanup(testServer.Close)
	t.Cleanup(func() { close(release) })

	requestDone := make(chan error, 1)
	client := &http.Client{Timeout: 5 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	go func() {
		response, requestErr := client.Get(testServer.URL)
		if response != nil {
			_ = response.Body.Close()
		}
		requestDone <- requestErr
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("test request did not reach the server")
	}

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- srv.shutdown() }()
	select {
	case err := <-shutdownDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("shutdown error = %v, want deadline exceeded", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("shutdown did not honor its configured one-second deadline")
	}
	select {
	case err := <-requestDone:
		if err == nil {
			t.Error("unfinished request was not terminated by forced close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not close the active connection")
	}
}

func TestShutdownWithoutActiveRequestsSucceeds(t *testing.T) {
	srv, err := Init(&config.HTTPConfig{HTTPSPort: 8443, ShutdownTimeoutSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	if err := srv.shutdown(); err != nil {
		t.Fatalf("shutdown without active requests: %v", err)
	}
}
