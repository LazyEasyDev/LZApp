package main

// import (
// 	"encoding/json"
// 	"net/http"
// 	"net/http/httptest"
// 	"strings"
// 	"testing"
// 	"time"
// )

// const (
// 	testJWTSecret = "test-jwt-secret-with-at-least-32-bytes"
// 	testUsername  = "test-user"
// 	testPassword  = "test-password"
// )

// func newTestHandler(t *testing.T) (http.Handler, *authService) {
// 	t.Helper()
// 	auth, err := newAuthService(testJWTSecret, testUsername, testPassword)
// 	if err != nil {
// 		t.Fatalf("create auth service: %v", err)
// 	}
// 	return newRouter(newUserStore(), auth), auth
// }

// func getAccessToken(t *testing.T, handler http.Handler) string {
// 	t.Helper()
// 	request := httptest.NewRequest(
// 		http.MethodPost,
// 		"/auth/token",
// 		strings.NewReader(`{"username":"test-user","password":"test-password"}`),
// 	)
// 	request.Header.Set("Content-Type", "application/json")
// 	response := httptest.NewRecorder()
// 	handler.ServeHTTP(response, request)

// 	if response.Code != http.StatusOK {
// 		t.Fatalf("expected token status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
// 	}

// 	var body struct {
// 		AccessToken string `json:"access_token"`
// 		TokenType   string `json:"token_type"`
// 		ExpiresIn   int64  `json:"expires_in"`
// 	}
// 	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
// 		t.Fatalf("decode token response: %v", err)
// 	}
// 	if body.AccessToken == "" || body.TokenType != "Bearer" || body.ExpiresIn != 3600 {
// 		t.Fatalf("unexpected token response: %+v", body)
// 	}
// 	return body.AccessToken
// }

// func TestHealth(t *testing.T) {
// 	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
// 	response := httptest.NewRecorder()

// 	handler, _ := newTestHandler(t)
// 	handler.ServeHTTP(response, request)

// 	if response.Code != http.StatusOK {
// 		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
// 	}
// 	if response.Body.String() != "ok" {
// 		t.Fatalf("expected body %q, got %q", "ok", response.Body.String())
// 	}
// }

// func TestUserFlow(t *testing.T) {
// 	handler, _ := newTestHandler(t)
// 	token := getAccessToken(t, handler)

// 	createRequest := httptest.NewRequest(
// 		http.MethodPost,
// 		"/users/create",
// 		strings.NewReader(`{"name":"Bob","email":"bob@example.com"}`),
// 	)
// 	createRequest.Header.Set("Content-Type", "application/json")
// 	createRequest.Header.Set("Authorization", "Bearer "+token)
// 	createResponse := httptest.NewRecorder()
// 	handler.ServeHTTP(createResponse, createRequest)

// 	if createResponse.Code != http.StatusOK {
// 		t.Fatalf("expected create status %d, got %d: %s", http.StatusOK, createResponse.Code, createResponse.Body.String())
// 	}
// 	if location := createResponse.Header().Get("Location"); location != "/v1/users/2" {
// 		t.Fatalf("expected location %q, got %q", "/v1/users/2", location)
// 	}

// 	getRequest := httptest.NewRequest(http.MethodGet, "/users/2?include_email=true", nil)
// 	getRequest.Header.Set("Authorization", "Bearer "+token)
// 	getResponse := httptest.NewRecorder()
// 	handler.ServeHTTP(getResponse, getRequest)

// 	if getResponse.Code != http.StatusOK {
// 		t.Fatalf("expected get status %d, got %d: %s", http.StatusOK, getResponse.Code, getResponse.Body.String())
// 	}
// 	if getResponse.Header().Get("X-Request-ID") == "" {
// 		t.Fatal("expected X-Request-ID response header")
// 	}

// 	var user User
// 	if err := json.NewDecoder(getResponse.Body).Decode(&user); err != nil {
// 		t.Fatalf("decode response: %v", err)
// 	}
// 	if user.ID != 2 || user.Name != "Bob" || user.Email != "bob@example.com" {
// 		t.Fatalf("unexpected user response: %+v", user)
// 	}
// }

// func TestProtectedRoutesRequireJWT(t *testing.T) {
// 	handler, _ := newTestHandler(t)
// 	tests := []struct {
// 		name          string
// 		authorization string
// 	}{
// 		{name: "missing token"},
// 		{name: "invalid token", authorization: "Bearer not-a-token"},
// 	}

// 	for _, test := range tests {
// 		t.Run(test.name, func(t *testing.T) {
// 			request := httptest.NewRequest(http.MethodGet, "/users/1", nil)
// 			if test.authorization != "" {
// 				request.Header.Set("Authorization", test.authorization)
// 			}
// 			response := httptest.NewRecorder()
// 			handler.ServeHTTP(response, request)

// 			if response.Code != http.StatusUnauthorized {
// 				t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, response.Code, response.Body.String())
// 			}
// 			if response.Header().Get("WWW-Authenticate") == "" {
// 				t.Fatal("expected WWW-Authenticate response header")
// 			}
// 		})
// 	}
// }

// func TestProtectedRoutesRejectInvalidClaims(t *testing.T) {
// 	handler, auth := newTestHandler(t)
// 	tests := []struct {
// 		name       string
// 		audience   []string
// 		expiration time.Time
// 	}{
// 		{
// 			name:       "expired token",
// 			audience:   []string{jwtAudience},
// 			expiration: time.Now().Add(-time.Minute),
// 		},
// 		{
// 			name:       "wrong audience",
// 			audience:   []string{"another-api"},
// 			expiration: time.Now().Add(time.Hour),
// 		},
// 	}

// 	for _, test := range tests {
// 		t.Run(test.name, func(t *testing.T) {
// 			_, token, err := auth.tokenAuth.Encode(map[string]any{
// 				"aud": test.audience,
// 				"exp": test.expiration.Unix(),
// 				"iss": jwtIssuer,
// 				"sub": testUsername,
// 			})
// 			if err != nil {
// 				t.Fatalf("encode token: %v", err)
// 			}

// 			request := httptest.NewRequest(http.MethodGet, "/users/1", nil)
// 			request.Header.Set("Authorization", "Bearer "+token)
// 			response := httptest.NewRecorder()
// 			handler.ServeHTTP(response, request)

// 			if response.Code != http.StatusUnauthorized {
// 				t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, response.Code, response.Body.String())
// 			}
// 		})
// 	}
// }

// func TestTokenRejectsInvalidCredentials(t *testing.T) {
// 	handler, _ := newTestHandler(t)
// 	request := httptest.NewRequest(
// 		http.MethodPost,
// 		"/auth/token",
// 		strings.NewReader(`{"username":"test-user","password":"wrong"}`),
// 	)
// 	request.Header.Set("Content-Type", "application/json")
// 	response := httptest.NewRecorder()
// 	handler.ServeHTTP(response, request)

// 	if response.Code != http.StatusUnauthorized {
// 		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, response.Code, response.Body.String())
// 	}
// }

// func TestUserValidation(t *testing.T) {
// 	request := httptest.NewRequest(http.MethodGet, "/users/0", nil)
// 	response := httptest.NewRecorder()

// 	handler, _ := newTestHandler(t)
// 	request.Header.Set("Authorization", "Bearer "+getAccessToken(t, handler))
// 	handler.ServeHTTP(response, request)

// 	if response.Code != http.StatusUnprocessableEntity {
// 		t.Fatalf("expected status %d, got %d: %s", http.StatusUnprocessableEntity, response.Code, response.Body.String())
// 	}
// }

// func TestOpenAPIDocument(t *testing.T) {
// 	request := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
// 	response := httptest.NewRecorder()

// 	handler, _ := newTestHandler(t)
// 	handler.ServeHTTP(response, request)

// 	if response.Code != http.StatusOK {
// 		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
// 	}

// 	var document struct {
// 		Paths      map[string]json.RawMessage `json:"paths"`
// 		Components struct {
// 			SecuritySchemes map[string]struct {
// 				Type         string `json:"type"`
// 				Scheme       string `json:"scheme"`
// 				BearerFormat string `json:"bearerFormat"`
// 			} `json:"securitySchemes"`
// 		} `json:"components"`
// 	}
// 	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
// 		t.Fatalf("decode OpenAPI document: %v", err)
// 	}
// 	for _, path := range []string{"/auth/token", "/users/create", "/users/{id}"} {
// 		if _, ok := document.Paths[path]; !ok {
// 			t.Errorf("OpenAPI document is missing path %q", path)
// 		}
// 	}
// 	if _, ok := document.Paths["/healthz"]; ok {
// 		t.Error("native chi health route should not appear in the OpenAPI document")
// 	}
// 	bearer := document.Components.SecuritySchemes["bearer"]
// 	if bearer.Type != "http" || bearer.Scheme != "bearer" || bearer.BearerFormat != "JWT" {
// 		t.Fatalf("unexpected bearer security scheme: %+v", bearer)
// 	}
// }

// func TestCreateUserOpenAPIExamples(t *testing.T) {
// 	request := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
// 	response := httptest.NewRecorder()

// 	handler, _ := newTestHandler(t)
// 	handler.ServeHTTP(response, request)

// 	var document struct {
// 		Components struct {
// 			Schemas map[string]struct {
// 				Properties map[string]struct {
// 					Examples []any `json:"examples"`
// 				} `json:"properties"`
// 			} `json:"schemas"`
// 		} `json:"components"`
// 	}
// 	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
// 		t.Fatalf("decode OpenAPI document: %v", err)
// 	}

// 	properties := document.Components.Schemas["CreateUserInputBody"].Properties
// 	if got := properties["name"].Examples; len(got) != 1 || got[0] != "Jane Doe" {
// 		t.Fatalf("unexpected name examples: %v", got)
// 	}
// 	if got := properties["email"].Examples; len(got) != 1 || got[0] != "jane@example.com" {
// 		t.Fatalf("unexpected email examples: %v", got)
// 	}
// }
