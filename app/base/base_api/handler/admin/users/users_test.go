package users

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	accounts "github.com/LazyEasyDev/LZApp/app/base/users"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

func TestRegisterRoutes(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("Test API", "1.0.0"))
	RegisterRoutes(api)
}

func TestPublicUserOmitsCredentials(t *testing.T) {
	name := "Member"
	account := &accounts.User{
		ID:        7,
		Name:      &name,
		Email:     "member@example.test",
		Password:  "secret-password-hash",
		ApiToken:  "secret-api-token",
		Access:    `["user","viewall"]`,
		CreatedAt: time.Unix(10, 0).UTC(),
		UpdatedAt: time.Unix(20, 0).UTC(),
	}

	result, err := publicUser(account)
	if err != nil {
		t.Fatalf("publicUser returned an error: %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal public user: %v", err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"password", "api_token", account.Password, account.ApiToken} {
		if strings.Contains(text, forbidden) {
			t.Errorf("public user contains forbidden credential %q: %s", forbidden, text)
		}
	}
	if result.Email != account.Email || len(result.Access) != 2 {
		t.Fatalf("public user lost expected fields: %#v", result)
	}
}

func TestEncodeAccessAllowsEmptyRejectsUnknownAndDeduplicates(t *testing.T) {
	empty, err := encodeAccess([]string{})
	if err != nil {
		t.Fatalf("encodeAccess rejected an empty access list: %v", err)
	}
	if empty != `[]` {
		t.Fatalf("encodeAccess(empty) = %q", empty)
	}
	if _, err := encodeAccess([]string{"unknown"}); err == nil {
		t.Fatal("encodeAccess accepted an unknown permission")
	}
	encoded, err := encodeAccess([]string{accounts.ACCESS_USER, accounts.ACCESS_ADMIN, accounts.ACCESS_USER})
	if err != nil {
		t.Fatalf("encodeAccess returned an error: %v", err)
	}
	if encoded != `["user","admin"]` {
		t.Fatalf("encodeAccess = %q", encoded)
	}
}

func TestValidatePasswordUsesCharactersAndUTF8Bytes(t *testing.T) {
	if err := validatePassword("12345678", true); err != nil {
		t.Fatalf("valid password rejected: %v", err)
	}
	if err := validatePassword("", false); err != nil {
		t.Fatalf("empty optional password rejected: %v", err)
	}
	if err := validatePassword("1234567", true); err == nil {
		t.Fatal("short password accepted")
	}
	if err := validatePassword(strings.Repeat("界", 25), true); err == nil {
		t.Fatal("password over 72 UTF-8 bytes accepted")
	}
}
