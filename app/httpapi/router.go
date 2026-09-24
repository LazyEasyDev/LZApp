package httpapi

import (
	"log/slog"
	"time"

	"github.com/LazyEasyDev/LZApp/app/httpapi/handler"
	"github.com/LazyEasyDev/LZApp/app/httpapi/middleware"
	"github.com/LazyEasyDev/LZApp/components"
	huma "github.com/danielgtaylor/huma/v2"
)

func registerRoutes(api huma.API) {
	api.UseMiddleware(middleware.ClientIPMiddleware(api))
	str, _ := components.GetComponents().Security.Generate()
	slog.Debug("example of a auth bear", "token", str)

	// Health check route
	slog.Debug("Registering health route")
	huma.Get(api, "/health", handler.HealthHandler)
	// Secure route requiring authentication
	huma.Get(api, "/auth", handler.AuthHandler,
		middleware.WithSpeedLimit(api, middleware.SpeedLimitPolicy{Requests: 1, Window: 10 * time.Second}),
		middleware.WithAuth(api, middleware.ValidateHMACToken),
	)

	// Additional routes can be registered here
}
