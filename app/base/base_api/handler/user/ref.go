package user

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/LazyEasyDev/LZApp/app/base/base_api/middleware"
	"github.com/LazyEasyDev/LZApp/app/base/users"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/LazyEasyDev/LZApp/components/security"
	"github.com/danielgtaylor/huma/v2"
)

type GenerateRefInput struct {
	Body struct {
		Tag string `json:"tag,omitempty" maxLength:"5" pattern:"^([a-z]{5})?$"`
	}
}

type GenerateRefOutput struct {
	Body struct {
		Ref string `json:"ref"`
	}
}

type ValidateRefInput struct {
	Ref string `query:"ref" maxLength:"128"`
}

type ValidateRefOutput struct {
	Body struct {
		Valid bool `json:"valid"`
	}
}

func registerRefRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "post-user-ref",
		Summary:     "Generate a referral token for the current user",
		Method:      http.MethodPost,
		Path:        "/user/ref",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{
			middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 30, Window: time.Minute}),
			middleware.UserAuthMiddleware(api, []string{users.ACCESS_USER}),
		},
	}, GenerateRefHandler)

	huma.Register(api, huma.Operation{
		OperationID: "get-user-ref",
		Summary:     "Validate a referral token",
		Method:      http.MethodGet,
		Path:        "/user/ref",
		Tags:        []string{"User"},
		Security:    []map[string][]string{},
		Middlewares: huma.Middlewares{
			middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 60, Window: time.Minute}),
		},
	}, ValidateRefHandler)
}

func GenerateRefHandler(ctx context.Context, input *GenerateRefInput) (*GenerateRefOutput, error) {
	account, authenticated := middleware.GetAuthUser(ctx)
	if !authenticated {
		return nil, huma.Error401Unauthorized("login required")
	}
	if !validReferralTag(input.Body.Tag) {
		return nil, huma.Error400BadRequest("referral tag must contain exactly 5 lowercase letters when provided")
	}
	token, err := components.GetTokenSigner().GenerateToken([]string{strconv.FormatUint(account.ID, 10), input.Body.Tag})
	if err != nil {
		return nil, huma.Error500InternalServerError("unable to generate referral token")
	}
	output := &GenerateRefOutput{}
	output.Body.Ref = token
	return output, nil
}

func ValidateRefHandler(ctx context.Context, input *ValidateRefInput) (*ValidateRefOutput, error) {
	ref, err := verifiedRef(ctx, input.Ref)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("referral validation is unavailable")
	}
	output := &ValidateRefOutput{}
	output.Body.Valid = ref != ""
	return output, nil
}

func validReferralTag(tag string) bool {
	if len(tag) != 0 && len(tag) != 5 {
		return false
	}
	for _, character := range tag {
		if character < 'a' || character > 'z' {
			return false
		}
	}
	return true
}

func referralValue(fields []string) (string, uint64, bool) {
	if len(fields) != 2 || !validReferralTag(fields[1]) {
		return "", 0, false
	}
	id, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil || id == 0 || strconv.FormatUint(id, 10) != fields[0] {
		return "", 0, false
	}
	return fields[0] + ":" + fields[1], id, true
}

func verifiedRef(ctx context.Context, token string) (string, error) {
	if token == "" || len(token) > 128 {
		return "", nil
	}
	fields, err := components.GetTokenSigner().VerifyToken(token)
	if errors.Is(err, security.ErrInvalidHMACToken) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	ref, id, valid := referralValue(fields)
	if !valid || ref == "" {
		return "", nil
	}
	var count int64
	if err := components.GetDB().WithContext(ctx).Model(&users.User{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return "", err
	}
	if count != 1 {
		return "", nil
	}
	return ref, nil
}
