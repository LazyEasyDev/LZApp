package middleware

import "github.com/danielgtaylor/huma/v2"

func NoStoreMiddleware() func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		ctx.SetHeader("Cache-Control", "no-store")
		next(ctx)
	}
}
