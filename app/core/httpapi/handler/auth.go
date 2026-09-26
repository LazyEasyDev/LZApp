package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/LazyEasyDev/LZApp/components"
	"github.com/LazyEasyDev/LZApp/config"
	"github.com/danielgtaylor/huma/v2"
)

type AuthOutput struct {
	SetCookie    http.Cookie `header:"Set-Cookie"`
	CacheControl string      `header:"Cache-Control"`

	Body struct {
		Verified       bool  `json:"verified" example:"true"`
		ServerUnixTime int64 `json:"server_unix_time" example:"1680000000"`
	}
}

func SetAuthHandler(_ context.Context, _ *struct{}) (*AuthOutput, error) {
	httpConfig := config.GetConfig().HTTP
	if httpConfig == nil || httpConfig.APITokenCookieName == "" {
		return nil, huma.Error500InternalServerError("cookie is not configured")
	}

	token, err := components.GetSecurity().Generate()
	if err != nil {
		return nil, huma.Error500InternalServerError("unable to generate token")
	}

	response := &AuthOutput{
		SetCookie: http.Cookie{
			Name:     httpConfig.APITokenCookieName,
			Value:    token,
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		},
		CacheControl: "no-store",
	}
	if err := response.SetCookie.Valid(); err != nil {
		return nil, huma.Error500InternalServerError("invalid cookie configuration")
	}
	response.Body.Verified = true
	response.Body.ServerUnixTime = time.Now().Unix()
	return response, nil
}

type AuthCheck struct {
	Body struct {
		Verified       bool  `json:"verified" example:"true"`
		ServerUnixTime int64 `json:"server_unix_time" example:"1680000000"`
	}
}

func AuthCheckHandler(ctx context.Context, _ *struct{}) (*AuthCheck, error) {
	response := &AuthCheck{}
	response.Body.Verified = true
	response.Body.ServerUnixTime = time.Now().Unix()
	return response, nil
}
