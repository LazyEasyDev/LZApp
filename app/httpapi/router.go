package httpapi

import (
	"log/slog"
	"time"

	"github.com/LazyEasyDev/LZApp/app/httpapi/handler"
	"github.com/LazyEasyDev/LZApp/app/httpapi/middleware"
	huma "github.com/danielgtaylor/huma/v2"
)

func registerRoutes(api huma.API) {
	api.UseMiddleware(middleware.ClientIPMiddleware(api))

	// Health check route
	slog.Debug("Registering health route")
	huma.Get(api, "/health", handler.HealthHandler, withSpeedLimit(api, speedLimitPolicy{Requests: 300, Window: 10 * time.Minute}))

	// Additional routes can be registered here
}
