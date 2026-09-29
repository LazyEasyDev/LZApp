package app

import (
	"log/slog"
	"net/http"

	"github.com/LazyEasyDev/LZApp/app/base/base_api/handler"
	"github.com/LazyEasyDev/LZApp/app/base/base_api/handler/docs"
	"github.com/LazyEasyDev/LZApp/app/base/base_api/handler/user"
	"github.com/LazyEasyDev/LZApp/app/base/base_api/middleware"
	huma "github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
)

func registerRoutes(api huma.API) {

	api.UseMiddleware(middleware.NoStoreMiddleware())
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

	// User routes
	user.RegisterRoutes(api)

	// Additional routes can be registered here

}
