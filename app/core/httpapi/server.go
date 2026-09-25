package httpapi

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/LazyEasyDev/LZApp/components"
)

func Start(ctx context.Context) error {
	runtime := components.GetComponents()
	if runtime.HTTP == nil {
		return fmt.Errorf("HTTP server is disabled or not initialized")
	}
	// register the HTTP routes
	slog.Info("Registering HTTP routes")
	registerRoutes(runtime.HTTP.API())
	// start the HTTP service
	slog.Info("Starting HTTPS service")
	return runtime.HTTP.Start(ctx)
}
