package http_server

import (
	"context"
	"strings"
	"testing"
)

func TestStartRequiresHTTPServer(t *testing.T) {
	err := Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP server is disabled or not initialized") {
		t.Fatalf("start error = %v, want missing HTTP server error", err)
	}
}
