package httpapi

import (
	"context"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
)

func registerRoutes(api huma.API) {
	slog.Debug("Registering health route")
	huma.Get(api, "/health", func(context.Context, *struct{}) (*healthOutput, error) {
		response := &healthOutput{}
		response.Body.Status = "ok"
		return response, nil
	})
}
