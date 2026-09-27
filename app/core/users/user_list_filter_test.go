package users

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/LazyEasyDev/LZApp/components"
	"gorm.io/gorm"
)

func TestUserListFiltersIntegration(test *testing.T) {
	initUserCacheIntegration(test)
	ctx := context.Background()
	database := components.GetDB()
	sqlDB, err := database.DB()
	if err != nil {
		test.Fatal(err)
	}
	connection, err := sqlDB.Conn(ctx)
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = connection.Close() })
	_, err = connection.ExecContext(ctx, `CREATE TEMPORARY TABLE user_list_filter_test (
		id BIGINT UNSIGNED PRIMARY KEY,
		name VARCHAR(100),
		email VARCHAR(254) NOT NULL,
		api_token VARCHAR(64) NOT NULL,
		access TEXT
	) CHARACTER SET utf8mb4`)
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() {
		if _, err := connection.ExecContext(ctx, "DROP TEMPORARY TABLE user_list_filter_test"); err != nil {
			test.Error(err)
		}
	})
	for _, row := range []struct {
		id       uint64
		name     any
		email    string
		apiToken string
		access   string
	}{
		{1, "Alice Johnson", "alice@example.com", "token-alpha", `["am"]`},
		{2, "Alicia", "alice2@example.com", "token-alpha-extra", `["am","admin"]`},
		{3, "Bob", "bobby@other.com", "token-beta", `["admin"]`},
		{4, nil, "no-name@example.com", "", `[]`},
		{5, `100%_!Team\A`, "percent%_!@example.com", "token-special", `["am"]`},
		{6, `100xxZ!Team\A`, "percentXY!@example.com", "token-other", `[]`},
		{7, "O'Neil", "oneil@other.com", "token'quoted", `[]`},
	} {
		if _, err := connection.ExecContext(ctx,
			"INSERT INTO user_list_filter_test (id, name, email, api_token, access) VALUES (?, ?, ?, ?, ?)",
			row.id, row.name, row.email, row.apiToken, row.access); err != nil {
			test.Fatal(err)
		}
	}
	callbackName := "test:user_list_filter_table"
	if err := database.Callback().Query().Before("gorm:query").Register(callbackName, func(query *gorm.DB) {
		query.Statement.Table = "user_list_filter_test"
		query.Statement.ConnPool = connection
	}); err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = database.Callback().Query().Remove(callbackName) })

	for _, scenario := range []struct {
		name   string
		filter ListFilter
		limit  int
		offset int
		ids    []uint64
	}{
		{name: "Unfiltered", ids: []uint64{1, 2, 3, 4, 5, 6, 7}},
		{name: "ExactID", filter: ListFilter{ID: 2}, ids: []uint64{2}},
		{name: "MissingID", filter: ListFilter{ID: 999}},
		{name: "NameSubstring", filter: ListFilter{Name: "lic"}, ids: []uint64{1, 2}},
		{name: "EmailSubstring", filter: ListFilter{Email: "lice"}, ids: []uint64{1, 2}},
		{name: "ExactApiToken", filter: ListFilter{ApiToken: "token-alpha"}, ids: []uint64{1}},
		{name: "TokenSubstringDoesNotMatch", filter: ListFilter{ApiToken: "alpha"}},
		{name: "ExactAccess", filter: ListFilter{Access: `["am"]`}, ids: []uint64{1, 5}},
		{name: "AccessSubstringDoesNotMatch", filter: ListFilter{Access: "am"}},
		{name: "AccessSpacingIsNotNormalized", filter: ListFilter{Access: `[ "am" ]`}},
		{name: "EmptyAccessArray", filter: ListFilter{Access: `[]`}, ids: []uint64{4, 6, 7}},
		{name: "AllFilters", filter: ListFilter{
			ID: 1, Name: "lic", Email: "lice", ApiToken: "token-alpha", Access: `["am"]`,
		}, ids: []uint64{1}},
		{name: "ANDMismatch", filter: ListFilter{ID: 2, ApiToken: "token-alpha"}},
		{name: "NameEmailAND", filter: ListFilter{Name: "lic", Email: "other.com"}},
		{name: "NameLiteralWildcards", filter: ListFilter{Name: "%_!"}, ids: []uint64{5}},
		{name: "EmailLiteralWildcards", filter: ListFilter{Email: "%_!"}, ids: []uint64{5}},
		{name: "Backslash", filter: ListFilter{Name: `Team\A`}, ids: []uint64{5, 6}},
		{name: "NameQuote", filter: ListFilter{Name: "O'Neil"}, ids: []uint64{7}},
		{name: "TokenQuote", filter: ListFilter{ApiToken: "token'quoted"}, ids: []uint64{7}},
		{name: "NameSQLInjection", filter: ListFilter{Name: "' OR 1=1 --"}},
		{name: "TokenSQLInjection", filter: ListFilter{ApiToken: "' OR 1=1 --"}},
		{name: "FilteredPagination", filter: ListFilter{Email: "example.com"}, limit: 2, offset: 1, ids: []uint64{2, 4}},
		{name: "NegativePagination", limit: -1, offset: -1, ids: []uint64{1, 2, 3, 4, 5, 6, 7}},
		{name: "PastEnd", offset: 100},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			users, err := List(ctx, scenario.filter, scenario.limit, scenario.offset)
			if err != nil {
				test.Fatal(err)
			}
			var ids []uint64
			for _, user := range users {
				ids = append(ids, user.ID)
			}
			if !reflect.DeepEqual(ids, scenario.ids) {
				test.Fatalf("IDs = %v, want %v", ids, scenario.ids)
			}
		})
	}

	test.Run("CanceledContext", func(test *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := List(ctx, ListFilter{}, 10, 0); !errors.Is(err, context.Canceled) {
			test.Fatalf("error = %v, want context.Canceled", err)
		}
	})
}
