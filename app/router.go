package app

import (
	"log/slog"

	"github.com/LazyEasyDev/LZApp/app/base/base_api/handler"
	admindbkv "github.com/LazyEasyDev/LZApp/app/base/base_api/handler/admin/dbkv"
	adminusers "github.com/LazyEasyDev/LZApp/app/base/base_api/handler/admin/users"
	"github.com/LazyEasyDev/LZApp/app/base/base_api/handler/docs"
	"github.com/LazyEasyDev/LZApp/app/base/base_api/handler/user"
	"github.com/LazyEasyDev/LZApp/app/base/base_api/middleware"
	huma "github.com/danielgtaylor/huma/v2"
)

func registerRoutes(api huma.API) {

	api.UseMiddleware(middleware.NoStoreMiddleware())
	api.UseMiddleware(middleware.ClientIPMiddleware(api))

	// Health check route never delte this line
	// for cloud server monitoring
	slog.Info("Registering health route")
	handler.RegisterHealthRoutes(api)

	// Api docs routes
	slog.Info("Registering docs routes")
	docs.RegisterRoutes(api)

	// Base api routes
	slog.Info("Registering base api routes")
	adminusers.RegisterRoutes(api)
	admindbkv.RegisterRoutes(api)
	user.RegisterRoutes(api)

	// Additional routes can be registered here

}
