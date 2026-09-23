package httpapi

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	cachelib "github.com/LazyEasyDev/LCache"
	"github.com/LazyEasyDev/LZApp/app/httpapi/middleware"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/danielgtaylor/huma/v2"
)

type speedLimitPolicy struct {
	Requests uint64
	Window   time.Duration
}

type requestCounter struct {
	count atomic.Uint64
}

func withSpeedLimit(api huma.API, policy speedLimitPolicy) func(*huma.Operation) {
	return withSpeedLimitCache(api, components.GetComponents().LCache, policy)
}

func withSpeedLimitCache(api huma.API, cache *cachelib.Cache, policy speedLimitPolicy) func(*huma.Operation) {
	if policy.Requests == 0 || policy.Window <= 0 || policy.Window%time.Second != 0 {
		panic("speed limit requires a positive request count and a positive whole-second window")
	}
	windowSeconds := int64(policy.Window / time.Second)

	return func(operation *huma.Operation) {
		route := operation.Method + " " + operation.Path
		operation.Middlewares = append(operation.Middlewares, func(ctx huma.Context, next func(huma.Context)) {
			retryAfterSeconds, err := consumeRequest(cache, route, middleware.GetClientIP(ctx.Context()), policy.Requests, windowSeconds)
			if err != nil {
				_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "request limiter unavailable")
				return
			}
			if retryAfterSeconds > 0 {
				ctx.SetHeader("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
				_ = huma.WriteErr(api, ctx, http.StatusTooManyRequests, "request limit exceeded")
				return
			}
			next(ctx)
		})
	}
}

func consumeRequest(cache *cachelib.Cache, route, clientIP string, requestLimit uint64, windowSeconds int64) (int64, error) {

	key := "http-rate-limit\\" + route + "\\" + clientIP
	value, _, remainingTTLSeconds, found := cache.GetWithTTL(key)
	retryAfterSeconds := max(remainingTTLSeconds+1, 1)
	counter, validCounter := value.(*requestCounter)
	if !found || !validCounter {
		counter = &requestCounter{}
		cache.Set(key, counter, windowSeconds)
		retryAfterSeconds = windowSeconds
	}
	if counter.count.Add(1) <= requestLimit {
		return 0, nil
	}

	return retryAfterSeconds, nil
}
