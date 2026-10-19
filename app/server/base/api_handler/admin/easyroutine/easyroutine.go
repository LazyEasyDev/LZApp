package easyroutine

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/LazyEasyDev/EasyRoutine"
	"github.com/LazyEasyDev/LZApp/app/server/base/middleware"
	"github.com/LazyEasyDev/LZApp/app/users"
	"github.com/danielgtaylor/huma/v2"
)

type ListInput struct{}

type ListOutput struct {
	Body struct {
		Items     []EasyRoutine.Supervisor `json:"items"`
		FetchedAt int64                    `json:"fetched_at"`
	}
}

type LogsInput struct {
	RoutineID string `query:"routine_id" required:"true" minLength:"64" maxLength:"64" pattern:"^[a-f0-9]{64}$"`
}

type LogsOutput struct {
	Body struct {
		Items     []EasyRoutine.SupervisorLog `json:"items"`
		FetchedAt int64                       `json:"fetched_at"`
	}
}

type DeleteInput struct {
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"255"`
	}
}

type DeleteOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

func RegisterRoutes(api huma.API) {
	readMiddleware := huma.Middlewares{
		middleware.UserAuthAnyAccessMiddleware(api, []string{users.AccessAdmin, users.AccessViewAll}),
		middleware.UserSpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 120, Window: time.Minute}),
	}
	writeMiddleware := huma.Middlewares{
		middleware.UserAuthMiddleware(api, []string{users.AccessAdmin}),
		middleware.UserSpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 30, Window: time.Minute}),
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-admin-easyroutine-list",
		Summary:     "List routine states and retained log counts",
		Method:      http.MethodGet,
		Path:        "/admin/easyroutine/list",
		Tags:        []string{"Admin EasyRoutine"},
		Middlewares: readMiddleware,
	}, ListHandler)
	huma.Register(api, huma.Operation{
		OperationID: "get-admin-easyroutine-logs",
		Summary:     "List retained logs for a routine",
		Method:      http.MethodGet,
		Path:        "/admin/easyroutine/logs",
		Tags:        []string{"Admin EasyRoutine"},
		Middlewares: readMiddleware,
	}, LogsHandler)
	huma.Register(api, huma.Operation{
		OperationID: "post-admin-easyroutine-delete",
		Summary:     "Delete a routine's stored state and retained history",
		Method:      http.MethodPost,
		Path:        "/admin/easyroutine/delete",
		Tags:        []string{"Admin EasyRoutine"},
		Middlewares: writeMiddleware,
	}, DeleteHandler)
}

func ListHandler(ctx context.Context, _ *ListInput) (*ListOutput, error) {
	supervisors, err := EasyRoutine.ListSupervisors(ctx)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("routine list is unavailable")
	}
	output := &ListOutput{}
	output.Body.Items = supervisors
	output.Body.FetchedAt = time.Now().Unix()
	return output, nil
}

func DeleteHandler(ctx context.Context, input *DeleteInput) (*DeleteOutput, error) {
	if err := EasyRoutine.DeleteSupervisorRecords(ctx, input.Body.Name); err != nil {
		if errors.Is(err, EasyRoutine.ErrSupervisorRegistered) {
			return nil, huma.Error409Conflict(err.Error())
		}
		return nil, huma.Error503ServiceUnavailable("routine deletion is unavailable")
	}
	output := &DeleteOutput{}
	output.Body.Message = "Routine records deleted"
	return output, nil
}

func LogsHandler(ctx context.Context, input *LogsInput) (*LogsOutput, error) {
	history, err := EasyRoutine.GetSupervisorHistory(ctx, input.RoutineID)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("routine logs are unavailable")
	}
	output := &LogsOutput{}
	output.Body.Items = history
	output.Body.FetchedAt = time.Now().Unix()
	return output, nil
}
