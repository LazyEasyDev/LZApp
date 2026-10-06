package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/LazyEasyDev/LZApp/app/base/users"
	"github.com/LazyEasyDev/LZApp/config"
	"github.com/danielgtaylor/huma/v2"
)

type authUserContextKey struct{}
type authAccessContextKey struct{}

func UserAuthMiddleware(api huma.API, requireAllAccessList []string) func(huma.Context, func(huma.Context)) {
	return userAuthMiddleware(api, slices.Clone(requireAllAccessList), nil)
}

func UserAuthAnyAccessMiddleware(api huma.API, requireAnyAccessList []string) func(huma.Context, func(huma.Context)) {
	return userAuthMiddleware(api, nil, slices.Clone(requireAnyAccessList))
}

func userAuthMiddleware(api huma.API, requireAllAccessList, requireAnyAccessList []string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		var tokens []string
		if token, valid := bearerToken(ctx); valid && len(token) <= 128 {
			tokens = append(tokens, token)
		}
		if settings := config.GetConfig().HTTP; settings != nil && settings.APITokenCookieName != "" {
			cookieToken, matches := "", 0
			for _, cookie := range huma.ReadCookies(ctx) {
				if cookie.Name == settings.APITokenCookieName {
					cookieToken = cookie.Value
					matches++
				}
			}
			if matches == 1 && cookieToken != "" && len(cookieToken) <= 128 && (len(tokens) == 0 || tokens[0] != cookieToken) {
				tokens = append(tokens, cookieToken)
			}
		}
		var lookupFailed bool
		for _, token := range tokens {
			account, notFound, err := ValidateUserToken(ctx.Context(), token)
			if err != nil {
				lookupFailed = true
				continue
			}
			if notFound {
				continue
			}
			if account != nil && account.ID != 0 {
				requestUser := *account
				var accessList []string
				if err := json.Unmarshal([]byte(requestUser.Access), &accessList); err != nil || accessList == nil {
					_ = huma.WriteErr(api, ctx, http.StatusForbidden, "invalid user access")
					return
				}
				ctx = huma.WithValue(ctx, authUserContextKey{}, &requestUser)
				ctx = huma.WithValue(ctx, authAccessContextKey{}, accessList)
				if len(requireAllAccessList) > 0 && !HaveAllAccess(ctx.Context(), requireAllAccessList) ||
					len(requireAnyAccessList) > 0 && !HaveAnyAccess(ctx.Context(), requireAnyAccessList) {
					_ = huma.WriteErr(api, ctx, http.StatusForbidden, "insufficient access")
					return
				}
				next(ctx)
				return
			}
		}
		if lookupFailed {
			_ = huma.WriteErr(api, ctx, http.StatusServiceUnavailable, "authentication is unavailable")
			return
		}
		ctx.SetHeader("WWW-Authenticate", "Bearer")
		_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "valid API token or login cookie required")
	}
}

func ValidateUserToken(ctx context.Context, token string) (*users.User, bool, error) {
	return users.GetByApiToken(ctx, token)
}

func GetAuthUser(ctx context.Context) (*users.User, bool) {
	account, ok := ctx.Value(authUserContextKey{}).(*users.User)
	return account, ok && account != nil
}

func HaveAllAccess(ctx context.Context, accessList []string) bool {
	granted, ok := ctx.Value(authAccessContextKey{}).([]string)
	if !ok || granted == nil {
		return false
	}
	for _, access := range accessList {
		if !slices.Contains(granted, access) {
			return false
		}
	}
	return true
}

func HaveAnyAccess(ctx context.Context, accessList []string) bool {
	granted, ok := ctx.Value(authAccessContextKey{}).([]string)
	if !ok || granted == nil {
		return false
	}
	for _, access := range accessList {
		if slices.Contains(granted, access) {
			return true
		}
	}
	return false
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
