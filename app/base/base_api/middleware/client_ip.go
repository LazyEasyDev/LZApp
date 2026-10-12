package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/LazyEasyDev/LZApp/config"
	"github.com/danielgtaylor/huma/v2"
)

type clientIPContextKey struct{}

func ClientIPMiddleware(api huma.API) func(huma.Context, func(huma.Context)) {
	var trustedProxies []netip.Prefix
	if settings := config.GetConfig().HTTP; settings != nil {
		for _, value := range settings.TrustedProxyCIDRs {
			if prefix, err := netip.ParsePrefix(value); err == nil {
				trustedProxies = append(trustedProxies, prefix)
			}
		}
	}
	return func(ctx huma.Context, next func(huma.Context)) {
		remoteAddress := ctx.RemoteAddr()
		var forwardedFor string
		ctx.EachHeader(func(name, value string) {
			if strings.EqualFold(name, "X-Forwarded-For") {
				forwardedFor = value
			}
		})
		clientIP := clientIPFromRequest(remoteAddress, forwardedFor, trustedProxies)
		if clientIP == "" {
			slog.Warn("Unable to determine client IP", "remote_address", remoteAddress)
			_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "unable to determine client IP")
			return
		}
		next(huma.WithValue(ctx, clientIPContextKey{}, clientIP))
	}
}

func GetClientIP(ctx context.Context) string {
	clientIP, _ := ctx.Value(clientIPContextKey{}).(string)
	return clientIP
}

func clientIPFromRequest(remoteAddress, forwardedFor string, trustedProxies []netip.Prefix) string {
	clientIP := clientIPFromRemoteAddress(remoteAddress)
	address, err := netip.ParseAddr(clientIP)
	if err != nil {
		return ""
	}
	for _, proxy := range trustedProxies {
		if proxy.Contains(address) {
			last := strings.LastIndexByte(forwardedFor, ',')
			forwarded := clientIPFromRemoteAddress(strings.TrimSpace(forwardedFor[last+1:]))
			if forwarded != "" {
				return forwarded
			}
			break
		}
	}
	return clientIP
}

func clientIPFromRemoteAddress(remoteAddress string) string {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = strings.Trim(remoteAddress, "[]")
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return ""
	}
	return address.Unmap().String()
}
