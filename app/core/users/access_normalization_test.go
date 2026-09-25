package users

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/LazyEasyDev/LZApp/components"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestNormalizeAccess(test *testing.T) {
	cases := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{name: "empty input", input: "", want: "[]"},
		{name: "empty array", input: "[]", want: "[]"},
		{name: "array whitespace", input: " \n[ ]\t", want: "[]"},
		{name: "all permissions", input: GetAccessListJsonStr(), want: `["admin","viewall","am"]`},
		{name: "single permission", input: `["viewall"]`, want: `["viewall"]`},
		{name: "duplicates", input: `["am","admin","am","viewall","admin"]`, want: `["am","admin","viewall"]`},
		{name: "compact JSON", input: ` [ "viewall", "viewall", "am" ] `, want: `["viewall","am"]`},
		{name: "escaped duplicate", input: `["\u0061dmin","admin"]`, want: `["admin"]`},
		{name: "invalid JSON", input: `broken`, wantError: true},
		{name: "whitespace only", input: " \n\t", wantError: true},
		{name: "null", input: `null`, wantError: true},
		{name: "object", input: `{}`, wantError: true},
		{name: "scalar string", input: `"admin"`, wantError: true},
		{name: "scalar number", input: `1`, wantError: true},
		{name: "number entry", input: `[1]`, wantError: true},
		{name: "boolean entry", input: `[true]`, wantError: true},
		{name: "null entry", input: `[null]`, wantError: true},
		{name: "mixed null", input: `["admin",null]`, wantError: true},
		{name: "object entry", input: `[{}]`, wantError: true},
		{name: "nested array", input: `[["admin"]]`, wantError: true},
		{name: "empty permission", input: `[""]`, wantError: true},
		{name: "unknown permission", input: `["unknown"]`, wantError: true},
		{name: "mixed unknown", input: `["admin","unknown","admin"]`, wantError: true},
		{name: "case mismatch", input: `["ADMIN"]`, wantError: true},
		{name: "padded permission", input: `[" admin "]`, wantError: true},
		{name: "trailing JSON", input: `[] []`, wantError: true},
	}
	for _, current := range cases {
		test.Run(current.name, func(test *testing.T) {
			got, err := normalizeAccess(current.input)
			if current.wantError {
				if err == nil || got != "" {
					test.Fatalf("expected rejection with no partial result: got %q, error=%v", got, err)
				}
				return
			}
			if err != nil || got != current.want {
				test.Fatalf("normalizeAccess: got %q, want %q, error=%v", got, current.want, err)
			}
			again, err := normalizeAccess(got)
			if err != nil || again != got {
				test.Fatalf("normalization is not idempotent: got %q, error=%v", again, err)
			}
		})
	}
}

func TestAccessCatalogConsistency(test *testing.T) {
	want := []string{ACCESS_ADMIN, ACCESS_VIEWALL, ACCESS_AM}
	if got := GetAccessList(); !reflect.DeepEqual(got, want) {
		test.Fatalf("access list: got %v, want %v", got, want)
	}
	var decoded []string
	if err := json.Unmarshal([]byte(GetAccessListJsonStr()), &decoded); err != nil {
		test.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, want) {
		test.Fatalf("JSON catalog: got %v, want %v", decoded, want)
	}
	copyOfList := GetAccessList()
	copyOfList[0] = "external"
	if !reflect.DeepEqual(GetAccessList(), want) {
		test.Fatal("caller changed the permission catalog")
	}
	if _, err := normalizeAccess(`["external"]`); err == nil {
		test.Fatal("caller mutation added an allowed permission")
	}
}

func newNormalizationTestDatabase(test *testing.T) *gorm.DB {
	test.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		test.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		test.Fatal(err)
	}
	connection.SetMaxOpenConns(1)
	previous := components.GetComponents().DB
	components.GetComponents().DB = database
	test.Cleanup(func() {
		components.GetComponents().DB = previous
		if err := connection.Close(); err != nil {
			test.Error(err)
		}
	})
	if err := CreateTable(test.Context()); err != nil {
		test.Fatal(err)
	}
	return database
}

func TestCreateAndUpdateNormalizeAccess(test *testing.T) {
	database := newNormalizationTestDatabase(test)
	ctx := test.Context()
	user := &User{
		Email:    "normalization@example.test",
		ApiToken: "normalization-test-token",
		Access:   `["am","admin","am"]`,
	}
	if err := Create(ctx, user); err != nil {
		test.Fatal(err)
	}
	stored, err := Get(ctx, user.ID)
	if err != nil || stored.Access != `["am","admin"]` || user.Access != stored.Access {
		test.Fatalf("Create did not store normalized access: user=%+v, error=%v", stored, err)
	}
	user.Access = `["viewall","am","viewall"]`
	if err := Update(ctx, user); err != nil {
		test.Fatal(err)
	}
	stored, err = Get(ctx, user.ID)
	if err != nil || stored.Access != `["viewall","am"]` || user.Access != stored.Access {
		test.Fatalf("Update did not store normalized access: user=%+v, error=%v", stored, err)
	}
	for _, access := range []string{`["admin","unknown"]`, `[null]`, `[""]`, `null`, `broken`} {
		invalid := &User{Email: "invalid@example.test", ApiToken: "invalid-test-token", Access: access}
		if err := Create(ctx, invalid); err == nil {
			test.Fatalf("Create accepted %s", access)
		}
		if invalid.Access != access || invalid.ID != 0 {
			test.Fatal("rejected Create changed the input user")
		}
		user.Access = access
		if err := Update(ctx, user); err == nil {
			test.Fatalf("Update accepted %s", access)
		}
		if user.Access != access {
			test.Fatal("rejected Update changed the input access")
		}
	}
	var count int64
	if err := database.Model(&User{}).Count(&count).Error; err != nil || count != 1 {
		test.Fatalf("rejected Create inserted rows: count=%d, error=%v", count, err)
	}
	stored, err = Get(ctx, user.ID)
	if err != nil || stored.Access != `["viewall","am"]` {
		test.Fatalf("rejected Update overwrote stored access: user=%+v, error=%v", stored, err)
	}
	user.Access = ""
	if err := Update(ctx, user); err != nil {
		test.Fatal(err)
	}
	stored, err = Get(ctx, user.ID)
	if err != nil || stored.Access != "[]" {
		test.Fatalf("empty access did not clear permissions: user=%+v, error=%v", stored, err)
	}
	unnamed := &User{Email: "empty-access@example.test", ApiToken: "empty-access-test-token"}
	if err := Create(ctx, unnamed); err != nil {
		test.Fatal(err)
	}
	stored, err = Get(ctx, unnamed.ID)
	if err != nil || stored.Access != "[]" {
		test.Fatalf("Create did not default empty access: user=%+v, error=%v", stored, err)
	}
}
