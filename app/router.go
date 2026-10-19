package app

import (
	"log/slog"

	"github.com/LazyEasyDev/LZApp/app/server/base/api_handler"
	admindbkv "github.com/LazyEasyDev/LZApp/app/server/base/api_handler/admin/dbkv"
	admineasyroutine "github.com/LazyEasyDev/LZApp/app/server/base/api_handler/admin/easyroutine"
	admineasystatistics "github.com/LazyEasyDev/LZApp/app/server/base/api_handler/admin/easystatistics"
	adminusers "github.com/LazyEasyDev/LZApp/app/server/base/api_handler/admin/users"
	"github.com/LazyEasyDev/LZApp/app/server/base/api_handler/am/refstatistics"
	"github.com/LazyEasyDev/LZApp/app/server/base/api_handler/docs"
	"github.com/LazyEasyDev/LZApp/app/server/base/api_handler/user"
	"github.com/LazyEasyDev/LZApp/app/server/base/middleware"
	huma "github.com/danielgtaylor/huma/v2"
)

func registerRoutes(api huma.API) {

	api.UseMiddleware(middleware.NoStoreMiddleware())
	api.UseMiddleware(middleware.ClientIPMiddleware(api))

	// Health check route never delte this line
	// for cloud server monitoring
	slog.Info("Registering health route")
	api_handler.RegisterHealthRoutes(api)

	// Api docs routes
	slog.Info("Registering docs routes")
	docs.RegisterRoutes(api)

	// Base api routes
	slog.Info("Registering base api routes")
	adminusers.RegisterRoutes(api)
	admindbkv.RegisterRoutes(api)
	admineasyroutine.RegisterRoutes(api)
	admineasystatistics.RegisterRoutes(api)
	user.RegisterRoutes(api)
	refstatistics.RegisterRoutes(api)

	// Additional routes can be registered here

}
