package middleware

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/LazyEasyDev/LCache"
	"github.com/danielgtaylor/huma/v2"
)

type SpeedLimitPolicy struct {
	Requests uint64
	Window   time.Duration
}

type requestCounter struct {
	count atomic.Uint64
}

func WithSpeedLimit(api huma.API, policy SpeedLimitPolicy) func(*huma.Operation) {
	return withSpeedLimitCache(api, policy)
}

func withSpeedLimitCache(api huma.API, policy SpeedLimitPolicy) func(*huma.Operation) {
	if policy.Requests == 0 || policy.Window <= 0 || policy.Window%time.Second != 0 {
		panic("speed limit requires a positive request count and a positive whole-second window")
	}
	windowSeconds := int64(policy.Window / time.Second)

	return func(operation *huma.Operation) {
		route := operation.Method + " " + operation.Path
		operation.Middlewares = append(operation.Middlewares, func(ctx huma.Context, next func(huma.Context)) {
			retryAfterSeconds, err := consumeRequest(route, GetClientIP(ctx.Context()), policy.Requests, windowSeconds)
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

func consumeRequest(route, clientIP string, requestLimit uint64, windowSeconds int64) (int64, error) {

	key := "http-rate-limit\\" + route + "\\" + clientIP
	value, _, remainingTTLSeconds, found := LCache.GetWithTTL(key)
	retryAfterSeconds := max(remainingTTLSeconds+1, 1)
	counter, validCounter := value.(*requestCounter)
	if !found || !validCounter {
		counter = &requestCounter{}
		LCache.Set(key, counter, windowSeconds)
		retryAfterSeconds = windowSeconds
	}
	if counter.count.Add(1) <= requestLimit {
		return 0, nil
	}

	return retryAfterSeconds, nil
}
