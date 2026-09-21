package http_server

import (
	"context"
	"log/slog"

	"github.com/LazyEasyDev/LZApp/components"
	"github.com/LazyEasyDev/LZApp/config"
)

func Start(ctx context.Context) {
	runtime := components.GetComponents()
	// register the HTTP routes
	slog.Info("Registering HTTP routes")
	registerRoutes(runtime.HTTP.API())
	// start the HTTP service
	slog.Info("Starting HTTPS service", "port", config.GetConfig().HTTP.HTTPSPort)
	service_start_err := runtime.HTTP.Start(ctx)
	if service_start_err != nil {
		slog.Error(service_start_err.Error())
	}
}
