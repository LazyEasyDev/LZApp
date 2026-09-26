package httpapi

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/LazyEasyDev/LZApp/components"
)

func Start(ctx context.Context) error {
	http_server := components.GetHTTP()
	if http_server == nil {
		return fmt.Errorf("HTTP server is disabled or not initialized")
	}
	// register the HTTP routes
	slog.Info("Registering HTTP routes")
	registerRoutes(http_server.API())
	// start the HTTP service
	slog.Info("Starting HTTP service")
	return http_server.Start(ctx)
}
