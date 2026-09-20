package httpserver

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/LazyEasyDev/LZApp/config"
)

func TestStartStopsWhenContextIsCanceled(t *testing.T) {
	server, err := Init(&config.HTTPConfig{HTTPSPort: 8443})
	if err != nil {
		t.Fatalf("initialize HTTPS server: %v", err)
	}
	server.server.Addr = "127.0.0.1:0"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Start(ctx)
	}()

	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("start HTTPS server: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTPS server did not stop after context cancellation")
	}
}

func TestSetErrorHandlerReceivesServerErrors(t *testing.T) {
	server, err := Init(&config.HTTPConfig{HTTPSPort: 8443})
	if err != nil {
		t.Fatalf("initialize HTTPS server: %v", err)
	}

	var received string
	var receivedMu sync.Mutex
	server.SetErrorHandler(func(message string) {
		receivedMu.Lock()
		defer receivedMu.Unlock()
		received = message
	})
	server.server.ErrorLog.Print("TLS handshake error")

	receivedMu.Lock()
	defer receivedMu.Unlock()
	if received != "TLS handshake error" {
		t.Fatalf("received error = %q, want TLS handshake error", received)
	}
}
