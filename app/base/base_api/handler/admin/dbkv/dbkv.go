package dbkv

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/LazyEasyDev/LZApp/app/base/base_api/middleware"
	store "github.com/LazyEasyDev/LZApp/app/base/dbkv"
	"github.com/LazyEasyDev/LZApp/app/base/users"
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type ListInput struct{}

type ListOutput struct {
	Body struct {
		Items []store.Entry `json:"items"`
	}
}

type RecordBody struct {
	Key         string `json:"key" minLength:"1" maxLength:"191"`
	Value       string `json:"value" minLength:"1"`
	Description string `json:"description"`
}

type CreateInput struct {
	Body RecordBody
}

type UpdateInput struct {
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
		middleware.UserAuthAnyAccessMiddleware(api, []string{users.ACCESS_ADMIN, users.ACCESS_VIEWALL}),
	}
	writeMiddleware := huma.Middlewares{
		middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 30, Window: time.Minute}),
		middleware.UserAuthMiddleware(api, []string{users.ACCESS_ADMIN}),
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
		OperationID:   "post-admin-dbkv-create",
		Summary:       "Create a DBKV record",
		Method:        http.MethodPost,
		Path:          "/admin/dbkv/create",
		DefaultStatus: http.StatusCreated,
		Tags:          []string{"Admin DBKV"},
		Middlewares:   writeMiddleware,
	}, CreateHandler)
	huma.Register(api, huma.Operation{
		OperationID: "post-admin-dbkv-update",
		Summary:     "Update a DBKV record",
		Method:      http.MethodPost,
		Path:        "/admin/dbkv/update",
		Tags:        []string{"Admin DBKV"},
		Middlewares: writeMiddleware,
	}, UpdateHandler)
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

func CreateHandler(ctx context.Context, input *CreateInput) (*WriteOutput, error) {
	key := strings.TrimSpace(input.Body.Key)
	if key == "" {
		return nil, huma.Error400BadRequest("key is required")
	}
	if strings.EqualFold(key, store.LastUpdatedKey) {
		return nil, huma.Error400BadRequest("key is reserved")
	}
	if !json.Valid([]byte(input.Body.Value)) {
		return nil, huma.Error400BadRequest("value must be valid JSON")
	}
	if err := store.Create(ctx, key, json.RawMessage(input.Body.Value), input.Body.Description); err != nil {
		var mysqlError *mysql.MySQLError
		if errors.Is(err, gorm.ErrDuplicatedKey) || errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return nil, huma.Error409Conflict("DBKV key already exists")
		}
		return nil, huma.Error503ServiceUnavailable("DBKV create is unavailable")
	}
	output := &WriteOutput{}
	output.Body.Message = "DBKV record created"
	return output, nil
}

func UpdateHandler(ctx context.Context, input *UpdateInput) (*WriteOutput, error) {
	if !json.Valid([]byte(input.Body.Value)) {
		return nil, huma.Error400BadRequest("value must be valid JSON")
	}
	entry, err := editableEntry(ctx, input.Body.Key)
	if err != nil {
		return nil, err
	}
	if err := store.Set(ctx, entry.Key, json.RawMessage(input.Body.Value), input.Body.Description); err != nil {
		return nil, huma.Error503ServiceUnavailable("DBKV update is unavailable")
	}
	output := &WriteOutput{}
	output.Body.Message = "DBKV record updated"
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
	entry, err := store.GetFromDB(ctx, key)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, huma.Error404NotFound("DBKV record not found")
	}
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("DBKV record is unavailable")
	}
	if !entry.Visible || entry.Key == store.LastUpdatedKey {
		return nil, huma.Error404NotFound("DBKV record not found")
	}
	return entry, nil
}
