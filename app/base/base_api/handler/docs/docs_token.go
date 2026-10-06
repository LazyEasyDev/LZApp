package docs

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/LazyEasyDev/LZApp/app/base/base_api/middleware"
	"github.com/LazyEasyDev/LZApp/app/base/users"
	"github.com/LazyEasyDev/LZApp/config"
)

func DocsTokenHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Vary", "Cookie, Origin, Sec-Fetch-Site")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	if request.Header.Get("X-LZApp-Docs") != "1" || !sameOriginDocsRequest(request) {
		http.Error(writer, "Forbidden", http.StatusForbidden)
		return
	}
	httpConfig := config.GetConfig().HTTP
	if httpConfig == nil || httpConfig.APITokenCookieName == "" {
		http.Error(writer, "Token unavailable", http.StatusServiceUnavailable)
		return
	}
	api_token := ""
	cookies := request.CookiesNamed(httpConfig.APITokenCookieName)
	if len(cookies) == 1 && cookies[0].Value != "" && len(cookies[0].Value) <= 128 {
		api_token = cookies[0].Value
	}

	account, _, err := middleware.ValidateUserToken(request.Context(), api_token)
	if err != nil {
		http.Error(writer, "Token unavailable", http.StatusServiceUnavailable)
		return
	}
	if account != nil && !account.HaveAllAccess([]string{users.ACCESS_USER}) {
		http.Error(writer, "User access required", http.StatusForbidden)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(DocsTokenViewHandler(account))
}

func sameOriginDocsRequest(request *http.Request) bool {
	if site := request.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		return false
	}
	origin := request.Header.Get("Origin")
	if origin == "" {
		return request.Header.Get("Sec-Fetch-Site") == "same-origin"
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	return parsed.Scheme == scheme && strings.EqualFold(parsed.Host, request.Host)
}
