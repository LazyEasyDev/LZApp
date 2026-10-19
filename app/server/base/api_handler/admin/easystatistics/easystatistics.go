package easystatistics

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/LazyEasyDev/EasyStatistics"
	"github.com/LazyEasyDev/LZApp/app/server/base/middleware"
	"github.com/LazyEasyDev/LZApp/app/users"
	"github.com/danielgtaylor/huma/v2"
)

type ListInput struct{}

type ListOutput struct {
	Body struct {
		Items           []EasyStatistics.StatisticsDefinition `json:"items"`
		TotalDBCount    int64                                 `json:"total_db_count"`
		ActiveDBCount   int64                                 `json:"active_db_count"`
		MaxQueryBuckets int                                   `json:"max_query_buckets"`
		FetchedAt       int64                                 `json:"fetched_at"`
	}
}

type BatchesInput struct{}

type BatchesOutput struct {
	Body struct {
		Count     int64 `json:"count"`
		FetchedAt int64 `json:"fetched_at"`
	}
}

type QueryInput struct {
	Body struct {
		Name          string                       `json:"name" minLength:"1" maxLength:"255"`
		Fields        []EasyStatistics.OrderField  `json:"fields" minItems:"1"`
		GroupInterval EasyStatistics.GroupInterval `json:"group_interval" enum:"DAY,WEEK,MONTH,YEAR,FOREVER"`
		Start         string                       `json:"start,omitempty" maxLength:"32"`
		End           string                       `json:"end,omitempty" maxLength:"32"`
	}
}

type QueryOutput struct {
	Body struct {
		Items     []EasyStatistics.Record `json:"items"`
		FetchedAt int64                   `json:"fetched_at"`
	}
}

func RegisterRoutes(api huma.API) {
	readMiddleware := huma.Middlewares{
		middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 120, Window: time.Minute}),
		middleware.UserAuthAnyAccessMiddleware(api, []string{users.AccessAdmin, users.AccessViewAll}),
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-admin-easystatistics-list",
		Summary:     "List locally registered statistics with persisted dimension row counts",
		Method:      http.MethodGet,
		Path:        "/admin/easystatistics/list",
		Tags:        []string{"Admin EasyStatistics"},
		Middlewares: readMiddleware,
	}, ListHandler)
	huma.Register(api, huma.Operation{
		OperationID: "get-admin-easystatistics-batches",
		Summary:     "Count globally stored statistics batch markers",
		Method:      http.MethodGet,
		Path:        "/admin/easystatistics/batches",
		Tags:        []string{"Admin EasyStatistics"},
		Middlewares: readMiddleware,
	}, BatchesHandler)
	huma.Register(api, huma.Operation{
		OperationID: "post-admin-easystatistics-query",
		Summary:     "Query stored buckets for exact dimension values",
		Method:      http.MethodPost,
		Path:        "/admin/easystatistics/query",
		Tags:        []string{"Admin EasyStatistics"},
		Middlewares: readMiddleware,
	}, QueryHandler)
}

func ListHandler(_ context.Context, _ *ListInput) (*ListOutput, error) {
	items, totalDBCount, activeDBCount, err := EasyStatistics.ListStatistics(true)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("statistics list is unavailable")
	}
	output := &ListOutput{}
	output.Body.Items = items
	output.Body.TotalDBCount = totalDBCount
	output.Body.ActiveDBCount = activeDBCount
	output.Body.MaxQueryBuckets = EasyStatistics.MaxQueryBuckets
	output.Body.FetchedAt = time.Now().Unix()
	return output, nil
}

func BatchesHandler(ctx context.Context, _ *BatchesInput) (*BatchesOutput, error) {
	count, err := EasyStatistics.CountBatches(ctx)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("statistics batch count is unavailable")
	}
	output := &BatchesOutput{}
	output.Body.Count = count
	output.Body.FetchedAt = time.Now().Unix()
	return output, nil
}

func QueryHandler(ctx context.Context, input *QueryInput) (*QueryOutput, error) {
	query, err := EasyStatistics.QueryStatistics(input.Body.Name)
	if errors.Is(err, EasyStatistics.ErrStatisticsNotFound) {
		return nil, huma.Error404NotFound("statistics is not registered in this process")
	}
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("statistics query is unavailable")
	}
	query.WithContext(ctx).GroupInterval(input.Body.GroupInterval)
	for _, field := range input.Body.Fields {
		query.Field(field.Name, field.Value)
	}
	if input.Body.End != "" {
		query.GroupTimeRange(input.Body.Start, input.Body.End)
	} else if input.Body.Start != "" {
		query.GroupTime(input.Body.Start)
	}
	items, err := query.Get()
	if errors.Is(err, EasyStatistics.ErrInvalidQuery) {
		return nil, huma.Error400BadRequest(err.Error())
	}
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("statistics query is unavailable")
	}
	output := &QueryOutput{}
	output.Body.Items = items
	output.Body.FetchedAt = time.Now().Unix()
	return output, nil
}
