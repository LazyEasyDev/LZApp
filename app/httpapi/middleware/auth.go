package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/LazyEasyDev/LZApp/components"
	"github.com/danielgtaylor/huma/v2"
)

type AuthIdentity struct {
	Subject string
}

type AuthHandler func(context.Context, string) (AuthIdentity, error)

type authIdentityContextKey struct{}

func ValidateHMACToken(_ context.Context, token string) (AuthIdentity, error) {
	payload, err := components.GetComponents().Security.Verify(token)
	if err != nil {
		return AuthIdentity{}, err
	}
	return AuthIdentity{Subject: payload}, nil
}

func WithAuth(api huma.API, validateToken AuthHandler) func(*huma.Operation) {
	authenticate := AuthMiddleware(api, validateToken)
	return func(operation *huma.Operation) {
		operation.Middlewares = append(operation.Middlewares, authenticate)
		operation.Security = []map[string][]string{{"bearerAuth": {}}}
	}
}

func AuthMiddleware(api huma.API, validateToken AuthHandler) func(huma.Context, func(huma.Context)) {
	if validateToken == nil {
		panic("authentication requires a token validator")
	}
	return func(ctx huma.Context, next func(huma.Context)) {
		token, valid := bearerToken(ctx)
		if !valid {
			ctx.SetHeader("WWW-Authenticate", "Bearer")
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "bearer token required")
			return
		}
		identity, err := validateToken(ctx.Context(), token)
		if err != nil || strings.TrimSpace(identity.Subject) == "" {
			ctx.SetHeader("WWW-Authenticate", `Bearer error="invalid_token"`)
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "invalid bearer token")
			return
		}
		next(huma.WithValue(ctx, authIdentityContextKey{}, identity))
	}
}

func GetAuthIdentity(ctx context.Context) (AuthIdentity, bool) {
	identity, authenticated := ctx.Value(authIdentityContextKey{}).(AuthIdentity)
	return identity, authenticated
}

func bearerToken(ctx huma.Context) (string, bool) {
	var authorization string
	headerCount := 0
	ctx.EachHeader(func(name, value string) {
		if strings.EqualFold(name, "Authorization") {
			authorization = value
			headerCount++
		}
	})
	if headerCount != 1 {
		return "", false
	}
	scheme, token, found := strings.Cut(authorization, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimLeft(token, " ")
	if token == "" {
		return "", false
	}
	padding := false
	for index, character := range token {
		if character == '=' && index > 0 {
			padding = true
			continue
		}
		if padding || !(character >= 'A' && character <= 'Z' ||
			character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			strings.ContainsRune("-._~+/", character)) {
			return "", false
		}
	}
	return token, true
}
