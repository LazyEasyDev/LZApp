package refstatistics

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/LazyEasyDev/LZApp/app/server/base/middleware"
	"github.com/LazyEasyDev/LZApp/app/users"
	"github.com/danielgtaylor/huma/v2"
)

type ListInput struct {
	RefUserID  uint64 `query:"ref_userid" minimum:"1"`
	RefChannel string `query:"ref_channel" maxLength:"32"`
	Page       int    `query:"page" minimum:"1" default:"1"`
	PageSize   int    `query:"page_size" minimum:"1" maximum:"100" default:"20"`
}

type ListOutput struct {
	Body struct {
		Items    []users.ReferralStatistic `json:"items"`
		Page     int                       `json:"page"`
		PageSize int                       `json:"page_size"`
		Total    int64                     `json:"total" doc:"Number of matching referrer and channel groups"`
	}
}

func RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-am-refstatistics-list",
		Summary:     "List registration counts by referring user and channel",
		Method:      http.MethodGet,
		Path:        "/am/refstatistics/list",
		Tags:        []string{"AM referral statistics"},
		Middlewares: huma.Middlewares{
			middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 60, Window: time.Minute}),
			middleware.UserAuthAnyAccessMiddleware(api, []string{users.ACCESS_ADMIN, users.ACCESS_AM, users.ACCESS_VIEWALL}),
		},
	}, ListHandler)
}

func ListHandler(ctx context.Context, input *ListInput) (*ListOutput, error) {
	account, authenticated := middleware.GetAuthUser(ctx)
	if !authenticated {
		return nil, huma.Error401Unauthorized("login required")
	}
	privileged := middleware.HaveAnyAccess(ctx, []string{users.ACCESS_ADMIN, users.ACCESS_VIEWALL})
	if !privileged && !middleware.HaveAnyAccess(ctx, []string{users.ACCESS_AM}) {
		return nil, huma.Error403Forbidden("insufficient access")
	}
	refUserID := input.RefUserID
	if !privileged {
		if refUserID != 0 && refUserID != account.ID {
			return nil, huma.Error403Forbidden("referral statistics are restricted to your own user ID")
		}
		refUserID = account.ID
	}
	if input.Page < 1 || input.PageSize < 1 || input.PageSize > 100 || input.Page > math.MaxInt/input.PageSize {
		return nil, huma.Error400BadRequest("invalid pagination")
	}
	items, total, err := users.ReferralStatistics(ctx, refUserID, input.RefChannel, input.PageSize, (input.Page-1)*input.PageSize)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("referral statistics are unavailable")
	}
	output := &ListOutput{}
	output.Body.Items = items
	output.Body.Page = input.Page
	output.Body.PageSize = input.PageSize
	output.Body.Total = total
	return output, nil
}
