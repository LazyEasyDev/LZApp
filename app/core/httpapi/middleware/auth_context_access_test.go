package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LazyEasyDev/LCache"
	"github.com/LazyEasyDev/LZApp/app/core/users"
	"github.com/LazyEasyDev/LZApp/config"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

func TestContextAccessHelpers(t *testing.T) {
	for _, test := range []struct {
		name      string
		granted   []string
		requested []string
		all       bool
		any       bool
	}{
		{name: "all granted", granted: []string{users.ACCESS_USER, users.ACCESS_ADMIN}, requested: []string{users.ACCESS_USER, users.ACCESS_ADMIN}, all: true, any: true},
		{name: "some granted", granted: []string{users.ACCESS_USER}, requested: []string{users.ACCESS_USER, users.ACCESS_ADMIN}, any: true},
		{name: "none granted", granted: []string{}, requested: []string{users.ACCESS_USER}},
		{name: "exact match", granted: []string{"superuser"}, requested: []string{users.ACCESS_USER}},
		{name: "empty request", granted: []string{}, all: true},
		{name: "duplicate request", granted: []string{users.ACCESS_USER}, requested: []string{users.ACCESS_USER, users.ACCESS_USER}, all: true, any: true},
		{name: "no auth context", requested: []string{users.ACCESS_USER}},
		{name: "no auth context with empty request"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			if test.granted != nil {
				ctx = context.WithValue(ctx, authAccessContextKey{}, test.granted)
			}
			if got := HaveAllAccess(ctx, test.requested); got != test.all {
				t.Errorf("HaveAllAccess() = %v, want %v", got, test.all)
			}
			if got := HaveAnyAccess(ctx, test.requested); got != test.any {
				t.Errorf("HaveAnyAccess() = %v, want %v", got, test.any)
			}
		})
	}
}

func TestUserAuthRequiredAccess(t *testing.T) {
	LCache.Init(LCache.Config{MaxTTLSeconds: 60})
	t.Cleanup(LCache.Close)
	for _, test := range []struct {
		name     string
		access   string
		required []string
		want     int
	}{
		{name: "user", access: `["user"]`, required: []string{users.ACCESS_USER}, want: http.StatusNoContent},
		{name: "initial admin", access: users.GetAccessListJsonStr(), required: []string{users.ACCESS_USER, users.ACCESS_ADMIN}, want: http.StatusNoContent},
		{name: "some required access missing", access: `["user"]`, required: []string{users.ACCESS_USER, users.ACCESS_ADMIN}, want: http.StatusForbidden},
		{name: "admin without user", access: `["admin"]`, required: []string{users.ACCESS_USER}, want: http.StatusForbidden},
		{name: "no access", access: `[]`, required: []string{users.ACCESS_USER}, want: http.StatusForbidden},
		{name: "no requirements", access: `[]`, want: http.StatusNoContent},
		{name: "empty access", access: "", required: []string{users.ACCESS_USER}, want: http.StatusForbidden},
		{name: "null access", access: `null`, required: []string{users.ACCESS_USER}, want: http.StatusForbidden},
		{name: "mixed types", access: `["user",1]`, required: []string{users.ACCESS_USER}, want: http.StatusForbidden},
		{name: "malformed access", access: `["user",`, required: []string{users.ACCESS_USER}, want: http.StatusForbidden},
		{name: "malformed without requirements", access: `["user",1]`, want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			account := &users.User{ID: 1, ApiToken: "context-access-token", Access: test.access}
			LCache.Set("user:api_token:"+account.ApiToken, account, 60)
			for _, credential := range []string{"bearer", "cookie", "missing", "unknown"} {
				t.Run(credential, func(t *testing.T) {
					router := chi.NewRouter()
					api := humachi.New(router, huma.DefaultConfig("test", "1"))
					called := false
					huma.Register(api, huma.Operation{
						OperationID: "auth-access-test", Method: http.MethodGet, Path: "/auth", DefaultStatus: http.StatusNoContent,
						Middlewares: huma.Middlewares{UserAuthMiddleware(api, test.required)},
					}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
						called = true
						authenticated, ok := GetAuthUser(ctx)
						if !ok || authenticated.ID != account.ID {
							t.Fatal("authenticated user missing from context")
						}
						authenticated.Access = `[]`
						if !HaveAllAccess(ctx, test.required) {
							t.Fatal("required permissions must remain in the decoded context")
						}
						if got := HaveAnyAccess(ctx, []string{users.ACCESS_USER, users.ACCESS_ADMIN}); got != (test.access != `[]`) {
							t.Fatalf("HaveAnyAccess() = %v for %s", got, test.access)
						}
						if HaveAnyAccess(ctx, []string{"unknown"}) {
							t.Fatal("context must not grant an absent permission")
						}
						return nil, nil
					})
					request := httptest.NewRequest(http.MethodGet, "/auth", nil)
					want := test.want
					switch credential {
					case "bearer":
						request.Header.Set("Authorization", "Bearer "+account.ApiToken)
					case "cookie":
						request.AddCookie(&http.Cookie{Name: config.GetConfig().HTTP.APITokenCookieName, Value: account.ApiToken})
					case "unknown":
						LCache.Set("user:api_token:unknown-token", (*users.User)(nil), 60)
						request.Header.Set("Authorization", "Bearer unknown-token")
						want = http.StatusUnauthorized
					case "missing":
						want = http.StatusUnauthorized
					}
					response := httptest.NewRecorder()
					router.ServeHTTP(response, request)
					if response.Code != want {
						t.Fatalf("status = %d, want %d: %s", response.Code, want, response.Body.String())
					}
					if called != (want == http.StatusNoContent) {
						t.Fatalf("handler called = %v", called)
					}
					if account.Access != test.access {
						t.Fatal("request context must not mutate the cached user")
					}
				})
			}
		})
	}
}
