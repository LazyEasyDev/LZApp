package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/LazyEasyDev/LZApp/app/core/httpapi/handler"
	"github.com/LazyEasyDev/LZApp/app/core/httpapi/handler/docs"
	"github.com/LazyEasyDev/LZApp/app/core/httpapi/middleware"
	huma "github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
)

func registerRoutes(api huma.API) {

	api.UseMiddleware(middleware.ClientIPMiddleware(api))

	// Docs routes
	docsHandler := docs.NewDocsHandler(api)
	api.Adapter().Handle(&huma.Operation{Method: http.MethodGet, Path: "/docs"}, func(ctx huma.Context) {
		request, writer := humachi.Unwrap(ctx)
		docsHandler.ServeHTTP(writer, request)
	})
	api.Adapter().Handle(&huma.Operation{Method: http.MethodPost, Path: "/docs_token"}, func(ctx huma.Context) {
		request, writer := humachi.Unwrap(ctx)
		docs.DocsTokenHandler(writer, request)
	})

	// Health check route
	slog.Debug("Registering health route")
	huma.Get(api, "/health", handler.HealthHandler)

	// Secure route requiring authentication
	huma.Get(api, "/auth/set", handler.SetAuthHandler)
	huma.Get(api, "/auth/check", handler.AuthCheckHandler,
		middleware.WithSpeedLimit(api, middleware.SpeedLimitPolicy{Requests: 1, Window: 10 * time.Second}),
		middleware.WithAuth(api, middleware.ValidateHMACToken))

	// Additional routes can be registered here

}
