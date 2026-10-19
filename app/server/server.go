package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/LazyEasyDev/LZApp/components"
)

func Start(ctx context.Context) error {
	httpServer := components.GetHTTP()
	if httpServer == nil {
		return fmt.Errorf("HTTP server is disabled or not initialized")
	}
	// register the HTTP routes
	slog.Info("Registering HTTP routes......")
	registerRoutes(httpServer.API())
	// start the HTTP service
	slog.Info("Starting HTTP service")
	return httpServer.Start(ctx)
}
