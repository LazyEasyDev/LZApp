package easyroutine

import (
	"context"
	"net/http"
	"time"

	"github.com/LazyEasyDev/LZApp/app/base/base_api/middleware"
	"github.com/LazyEasyDev/LZApp/app/base/users"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/danielgtaylor/huma/v2"
)

type LogCounts struct {
	NotStarted int64 `json:"not_started"`
	Running    int64 `json:"running"`
	Done       int64 `json:"done"`
	Panic      int64 `json:"panic"`
	Other      int64 `json:"other"`
}

type Routine struct {
	RoutineID     string    `json:"routine_id"`
	Name          string    `json:"name"`
	Owner         string    `json:"owner"`
	Status        string    `json:"status"`
	SuccessCount  int64     `json:"success_count"`
	FailureCount  int64     `json:"failure_count"`
	ExpiresAt     int64     `json:"expires_at"`
	UpdatedAt     int64     `json:"updated_at"`
	LeaseActive   bool      `json:"lease_active"`
	RetainedLogs  int64     `json:"retained_logs"`
	LatestPanicAt int64     `json:"latest_panic_at"`
	LogCounts     LogCounts `json:"log_counts" gorm:"-"`
}

type RoutineLog struct {
	ID        string `json:"id"`
	RoutineID string `json:"routine_id"`
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	Action    string `json:"action"`
	Status    string `json:"status"`
	Log       string `json:"log"`
	CreatedAt int64  `json:"created_at"`
}

type ListInput struct{}

type ListOutput struct {
	Body struct {
		Items     []Routine `json:"items"`
		FetchedAt int64     `json:"fetched_at"`
	}
}

type LogsInput struct {
	RoutineID string `query:"routine_id" required:"true" minLength:"64" maxLength:"64" pattern:"^[a-f0-9]{64}$"`
}

type LogsOutput struct {
	Body struct {
		Items     []RoutineLog `json:"items"`
		FetchedAt int64        `json:"fetched_at"`
	}
}

func RegisterRoutes(api huma.API) {
	readMiddleware := huma.Middlewares{
		middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 120, Window: time.Minute}),
		middleware.UserAuthAnyAccessMiddleware(api, []string{users.ACCESS_ADMIN, users.ACCESS_VIEWALL}),
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
}

func ListHandler(ctx context.Context, _ *ListInput) (*ListOutput, error) {
	db := components.GetDB().WithContext(ctx)
	items := make([]Routine, 0)
	err := db.Table("easy_rountine_unique").
		Select(
			"routine_id", "name", "owner", "status", "success_count", "failure_count",
			"expires_at", "updated_at", "expires_at > UNIX_TIMESTAMP() AS lease_active",
		).
		Order("name, routine_id").
		Find(&items).Error
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("routine list is unavailable")
	}
	var summaries []struct {
		RoutineID string
		Status    string
		Count     int64
		LatestAt  int64
	}
	err = db.Table("easy_routine_unique_log").
		Select("routine_id, status, COUNT(*) AS count, MAX(created_at) AS latest_at").
		Group("routine_id, status").
		Find(&summaries).Error
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("routine list is unavailable")
	}
	routinesByID := make(map[string]*Routine, len(items))
	for index := range items {
		routinesByID[items[index].RoutineID] = &items[index]
	}
	for _, summary := range summaries {
		routine := routinesByID[summary.RoutineID]
		if routine == nil {
			continue
		}
		routine.RetainedLogs += summary.Count
		switch summary.Status {
		case "not_started":
			routine.LogCounts.NotStarted += summary.Count
		case "running":
			routine.LogCounts.Running += summary.Count
		case "done":
			routine.LogCounts.Done += summary.Count
		case "panic":
			routine.LogCounts.Panic += summary.Count
			if summary.LatestAt > routine.LatestPanicAt {
				routine.LatestPanicAt = summary.LatestAt
			}
		default:
			routine.LogCounts.Other += summary.Count
		}
	}
	output := &ListOutput{}
	output.Body.Items = items
	output.Body.FetchedAt = time.Now().Unix()
	return output, nil
}

func LogsHandler(ctx context.Context, input *LogsInput) (*LogsOutput, error) {
	items := make([]RoutineLog, 0)
	err := components.GetDB().WithContext(ctx).
		Table("easy_routine_unique_log").
		Select("id", "routine_id", "name", "owner", "action", "status", "log", "created_at").
		Where("routine_id = ?", input.RoutineID).
		Order("created_at DESC, id DESC").
		Find(&items).Error
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("routine logs are unavailable")
	}
	output := &LogsOutput{}
	output.Body.Items = items
	output.Body.FetchedAt = time.Now().Unix()
	return output, nil
}
