package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

type clientIPContextKey struct{}

func ClientIPMiddleware(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		remoteAddress := ctx.RemoteAddr()
		clientIP := clientIPFromRemoteAddress(remoteAddress)
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
