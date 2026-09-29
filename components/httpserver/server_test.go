package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LazyEasyDev/LZApp/config/http_config"
)

func TestCORSPreflightDefaultMethods(t *testing.T) {
	server, err := New(&http_config.HTTPConfig{
		HTTPSPort:              8443,
		ShutdownTimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatalf("create HTTP server: %v", err)
	}

	const origin = "https://127.0.0.1:4173"
	for _, test := range []struct {
		name          string
		method        string
		allowedOrigin string
	}{
		{name: "post", method: http.MethodPost, allowedOrigin: origin},
		{name: "put", method: http.MethodPut},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodOptions, "/admin/users/3", nil)
			request.Header.Set("Origin", origin)
			request.Header.Set("Access-Control-Request-Method", test.method)
			request.Header.Set("Access-Control-Request-Headers", "content-type")
			response := httptest.NewRecorder()

			server.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
			}
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.allowedOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, test.allowedOrigin)
			}
		})
	}
}
