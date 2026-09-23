package httpapi

import (
	"context"
	"log/slog"
	"time"

	huma "github.com/danielgtaylor/huma/v2"
)

func registerRoutes(api huma.API) {
	slog.Debug("Registering health route")
	huma.Get(api, "/health", func(context.Context, *struct{}) (*healthOutput, error) {
		response := &healthOutput{}
		response.Body.ServerUnixTime = time.Now().Unix()
		return response, nil
	})
}
