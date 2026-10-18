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
		Channel string `json:"channel" minLength:"3" maxLength:"32" pattern:"^[A-Za-z0-9][A-Za-z0-9_-]{1,30}[A-Za-z0-9]$"`
	}
}

type GenerateRefOutput struct {
	Body struct {
		Ref string `json:"ref"`
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

}

func GenerateRefHandler(ctx context.Context, input *GenerateRefInput) (*GenerateRefOutput, error) {
	account, authenticated := middleware.GetAuthUser(ctx)
	if !authenticated {
		return nil, huma.Error401Unauthorized("login required")
	}
	if !validReferralChannel(input.Body.Channel) {
		return nil, huma.Error400BadRequest("channel must be 3-32 ASCII letters, digits, hyphens or underscores, starting and ending with a letter or digit")
	}
	token, err := components.GetTokenSigner().GenerateToken([]string{strconv.FormatUint(account.ID, 10), input.Body.Channel})
	if err != nil {
		return nil, huma.Error500InternalServerError("unable to generate referral token")
	}
	output := &GenerateRefOutput{}
	output.Body.Ref = token
	return output, nil
}

func validReferralChannel(channel string) bool {
	if len(channel) < 3 || len(channel) > 32 {
		return false
	}
	for index, character := range channel {
		alphanumeric := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9'
		if !alphanumeric && (index == 0 || index == len(channel)-1 || character != '-' && character != '_') {
			return false
		}
	}
	return true
}

func referralValue(fields []string) (uint64, string, bool) {
	if len(fields) != 2 || !validReferralChannel(fields[1]) {
		return 0, "", false
	}
	id, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil || id == 0 || strconv.FormatUint(id, 10) != fields[0] {
		return 0, "", false
	}
	return id, fields[1], true
}

func verifiedRef(ctx context.Context, token string) (uint64, string, error) {
	if token == "" || len(token) > 128 {
		return 0, "", nil
	}
	fields, err := components.GetTokenSigner().VerifyToken(token)
	if errors.Is(err, security.ErrInvalidHMACToken) {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	id, channel, valid := referralValue(fields)
	if !valid {
		return 0, "", nil
	}
	var count int64
	if err := components.GetDB().WithContext(ctx).Model(&users.User{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return 0, "", err
	}
	if count != 1 {
		return 0, "", nil
	}
	return id, channel, nil
}
