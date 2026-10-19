package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/LazyEasyDev/LCache"
	"github.com/LazyEasyDev/LZApp/common/cache_helper"
	"github.com/danielgtaylor/huma/v2"
)

type SpeedLimitPolicy struct {
	Requests uint64
	Window   time.Duration
}

type requestCounter struct {
	count uint64
}

var counterMu sync.Mutex

func IPSpeedLimitMiddleware(api huma.API, policy SpeedLimitPolicy) func(huma.Context, func(huma.Context)) {
	return speedLimitMiddleware(api, policy, false)
}

// UserSpeedLimitMiddleware must run after authentication.
func UserSpeedLimitMiddleware(api huma.API, policy SpeedLimitPolicy) func(huma.Context, func(huma.Context)) {
	return speedLimitMiddleware(api, policy, true)
}

func speedLimitMiddleware(api huma.API, policy SpeedLimitPolicy, byUser bool) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if policy.Requests == 0 || policy.Window <= 0 || policy.Window%time.Second != 0 {
			slog.Error("invalid speed limit policy", "requests", policy.Requests, "window", policy.Window)
			_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "request limiter unavailable")
			return
		}
		windowSeconds := int64(policy.Window / time.Second)
		operation := ctx.Operation()
		route := operation.Method + " " + operation.Path
		item := cache_helper.NewItem("http-rate-limit").Attr("route", route)
		if byUser {
			account, authenticated := GetAuthUser(ctx.Context())
			if !authenticated || account.ID == 0 {
				_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "login required")
				return
			}
			item.Attr("user_id", account.ID)
		} else {
			clientIP := GetClientIP(ctx.Context())
			if clientIP == "" {
				_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "request limiter unavailable")
				return
			}
			item.Attr("client_ip", clientIP)
		}
		retryAfterSeconds := consumeRequest(item, policy.Requests, windowSeconds)
		if retryAfterSeconds > 0 {
			ctx.SetHeader("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
			_ = huma.WriteErr(api, ctx, http.StatusTooManyRequests, "request limit exceeded")
			return
		}
		next(ctx)
	}
}

func consumeRequest(item *cache_helper.CacheItem, requestLimit uint64, windowSeconds int64) int64 {
	counterMu.Lock()
	defer counterMu.Unlock()
	result := LCache.Get(item.GetKey())
	retryAfterSeconds := max(result.RemainingSeconds(), 1)
	counter, validCounter := result.Val().(*requestCounter)
	if !result.Found() || !validCounter {
		counter = &requestCounter{}
		item.Val(counter).Set(windowSeconds)
		retryAfterSeconds = windowSeconds
	}
	counter.count++
	if counter.count <= requestLimit {
		return 0
	}

	return retryAfterSeconds
}
