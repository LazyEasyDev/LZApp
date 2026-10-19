package dbkv

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	store "github.com/LazyEasyDev/LZApp/app/dbkv"
	"github.com/LazyEasyDev/LZApp/app/server/base/middleware"
	"github.com/LazyEasyDev/LZApp/app/users"
	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"
)

type ListInput struct{}

type ListOutput struct {
	Body struct {
		Items []store.Entry `json:"items"`
	}
}

type RecordBody struct {
	Key         string       `json:"key" minLength:"1" maxLength:"191"`
	Format      store.Format `json:"format" enum:"int64,bool,float64,json_str,string"`
	Value       string       `json:"value"`
	Description string       `json:"description"`
}

type SetInput struct {
	Body RecordBody
}

type DeleteInput struct {
	Body struct {
		Key string `json:"key" minLength:"1" maxLength:"191"`
	}
}

type WriteOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

func RegisterRoutes(api huma.API) {
	readMiddleware := huma.Middlewares{
		middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 120, Window: time.Minute}),
		middleware.UserAuthAnyAccessMiddleware(api, []string{users.AccessAdmin, users.AccessViewAll}),
	}
	writeMiddleware := huma.Middlewares{
		middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 30, Window: time.Minute}),
		middleware.UserAuthMiddleware(api, []string{users.AccessAdmin}),
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-admin-dbkv-list",
		Summary:     "List all visible DBKV records",
		Method:      http.MethodGet,
		Path:        "/admin/dbkv/list",
		Tags:        []string{"Admin DBKV"},
		Middlewares: readMiddleware,
	}, ListHandler)
	huma.Register(api, huma.Operation{
		OperationID: "post-admin-dbkv-set",
		Summary:     "Set a DBKV record without changing its format",
		Method:      http.MethodPost,
		Path:        "/admin/dbkv/set",
		Tags:        []string{"Admin DBKV"},
		Middlewares: writeMiddleware,
	}, SetHandler)
	huma.Register(api, huma.Operation{
		OperationID: "post-admin-dbkv-del",
		Summary:     "Delete a DBKV record",
		Method:      http.MethodPost,
		Path:        "/admin/dbkv/del",
		Tags:        []string{"Admin DBKV"},
		Middlewares: writeMiddleware,
	}, DeleteHandler)
}

func ListHandler(ctx context.Context, _ *ListInput) (*ListOutput, error) {
	if err := store.Refresh(ctx); err != nil {
		return nil, huma.Error503ServiceUnavailable("DBKV list is unavailable")
	}
	items, err := store.List(ctx)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("DBKV list is unavailable")
	}
	output := &ListOutput{}
	output.Body.Items = items
	return output, nil
}

func SetHandler(ctx context.Context, input *SetInput) (*WriteOutput, error) {
	key := strings.TrimSpace(input.Body.Key)
	if key == "" || !utf8.ValidString(key) || utf8.RuneCountInString(key) > 191 {
		return nil, huma.Error400BadRequest("key must contain 1-191 valid UTF-8 characters")
	}
	if strings.EqualFold(key, store.LastUpdatedKey) {
		return nil, huma.Error400BadRequest("key is reserved")
	}
	entry, exists, err := store.GetRecordFromDB(ctx, key)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("DBKV record is unavailable")
	}
	if exists && !entry.Visible {
		return nil, huma.Error404NotFound("DBKV record not found")
	}
	err = store.SetRecord(ctx, store.Entry{
		Key:         key,
		Format:      input.Body.Format,
		Value:       input.Body.Value,
		Description: input.Body.Description,
	})
	if errors.Is(err, store.ErrInvalidValue) {
		return nil, huma.Error400BadRequest(err.Error())
	}
	if errors.Is(err, store.ErrFormatMismatch) {
		return nil, huma.Error409Conflict(err.Error())
	}
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("DBKV save is unavailable")
	}
	output := &WriteOutput{}
	output.Body.Message = "DBKV record saved"
	return output, nil
}

func DeleteHandler(ctx context.Context, input *DeleteInput) (*WriteOutput, error) {
	entry, err := editableEntry(ctx, input.Body.Key)
	if err != nil {
		return nil, err
	}
	if err := store.Delete(ctx, entry.Key); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, huma.Error404NotFound("DBKV record not found")
	} else if err != nil {
		return nil, huma.Error503ServiceUnavailable("DBKV delete is unavailable")
	}
	output := &WriteOutput{}
	output.Body.Message = "DBKV record deleted"
	return output, nil
}

func editableEntry(ctx context.Context, key string) (*store.Entry, error) {
	if strings.TrimSpace(key) == "" {
		return nil, huma.Error400BadRequest("key is required")
	}
	entry, exists, err := store.GetRecordFromDB(ctx, key)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("DBKV record is unavailable")
	}
	if !exists || !entry.Visible || entry.Key == store.LastUpdatedKey {
		return nil, huma.Error404NotFound("DBKV record not found")
	}
	return entry, nil
}
