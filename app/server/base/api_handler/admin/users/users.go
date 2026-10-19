package users

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/LazyEasyDev/LZApp/app/server/base/middleware"
	accounts "github.com/LazyEasyDev/LZApp/app/users"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type PublicUser struct {
	ID         uint64     `json:"id"`
	Name       *string    `json:"name"`
	Email      string     `json:"email"`
	Access     []string   `json:"access"`
	RefUserID  *uint64    `json:"ref_userid"`
	RefChannel *string    `json:"ref_channel"`
	LastLogin  *time.Time `json:"last_login"`
	RegIP      string     `json:"reg_ip"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type ListInput struct {
	ID         uint64 `query:"id" minimum:"1"`
	Name       string `query:"name" maxLength:"100"`
	Email      string `query:"email" maxLength:"254"`
	Access     string `query:"access" enum:"user,admin,viewall,am"`
	RefUserID  uint64 `query:"ref_userid" minimum:"1"`
	RefChannel string `query:"ref_channel" maxLength:"32"`
	RegIP      string `query:"reg_ip" maxLength:"45"`
	Page       int    `query:"page" minimum:"1" default:"1"`
	PageSize   int    `query:"page_size" minimum:"1" maximum:"100" default:"20"`
}

type ListOutput struct {
	Body struct {
		Items         []PublicUser `json:"items"`
		AccessOptions []string     `json:"access_options"`
		Page          int          `json:"page"`
		PageSize      int          `json:"page_size"`
		Total         int64        `json:"total"`
	}
}

type UserBody struct {
	Name     *string  `json:"name,omitempty" maxLength:"100"`
	Email    string   `json:"email" minLength:"3" maxLength:"254"`
	Password string   `json:"password,omitempty" maxLength:"72"`
	Access   []string `json:"access"`
}

type CreateInput struct {
	Body UserBody
}

type UpdateInput struct {
	Body struct {
		ID uint64 `json:"id" minimum:"1"`
		UserBody
	}
}

type UserOutput struct {
	Body PublicUser
}

func RegisterRoutes(api huma.API) {
	readAccess := []string{accounts.AccessAdmin, accounts.AccessViewAll}
	readMiddleware := huma.Middlewares{
		middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 120, Window: time.Minute}),
		middleware.UserAuthAnyAccessMiddleware(api, readAccess),
	}
	writeMiddleware := huma.Middlewares{
		middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 30, Window: time.Minute}),
		middleware.UserAuthMiddleware(api, []string{accounts.AccessAdmin}),
	}

	huma.Register(api, huma.Operation{
		OperationID: "get-admin-users-list",
		Summary:     "List users",
		Method:      http.MethodGet,
		Path:        "/admin/users/list",
		Tags:        []string{"Admin users"},
		Middlewares: readMiddleware,
	}, ListHandler)

	huma.Register(api, huma.Operation{
		OperationID:   "post-admin-users-create",
		Summary:       "Create user",
		Method:        http.MethodPost,
		Path:          "/admin/users/create",
		Tags:          []string{"Admin users"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   writeMiddleware,
	}, CreateHandler)

	huma.Register(api, huma.Operation{
		OperationID: "post-admin-users-update",
		Summary:     "Update user",
		Method:      http.MethodPost,
		Path:        "/admin/users/update",
		Tags:        []string{"Admin users"},
		Middlewares: writeMiddleware,
	}, UpdateHandler)
}

func ListHandler(ctx context.Context, input *ListInput) (*ListOutput, error) {
	if input.RefChannel != "" && input.RefUserID == 0 {
		return nil, huma.Error400BadRequest("referring user ID is required when filtering by referral channel")
	}
	if input.Page > math.MaxInt/input.PageSize {
		return nil, huma.Error400BadRequest("page is too large")
	}
	filter := accounts.ListFilter{
		Name:       strings.TrimSpace(input.Name),
		Email:      strings.TrimSpace(input.Email),
		Access:     input.Access,
		RefUserID:  input.RefUserID,
		RefChannel: input.RefChannel,
		RegIP:      strings.TrimSpace(input.RegIP),
	}
	if input.ID != 0 {
		filter.ID = input.ID
	}
	total, err := accounts.Count(ctx, filter)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("user list is unavailable")
	}
	rows, err := accounts.List(ctx, filter, input.PageSize, (input.Page-1)*input.PageSize)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("user list is unavailable")
	}
	items := make([]PublicUser, 0, len(rows))
	for index := range rows {
		item, err := publicUser(&rows[index])
		if err != nil {
			return nil, huma.Error500InternalServerError("stored user access is invalid")
		}
		items = append(items, item)
	}
	output := &ListOutput{}
	output.Body.Items = items
	output.Body.AccessOptions = accounts.GetAccessList()
	output.Body.Page = input.Page
	output.Body.PageSize = input.PageSize
	output.Body.Total = total
	return output, nil
}

func CreateHandler(ctx context.Context, input *CreateInput) (*UserOutput, error) {
	if err := validatePassword(input.Body.Password, true); err != nil {
		return nil, err
	}
	email, err := normalizeEmail(input.Body.Email)
	if err != nil {
		return nil, err
	}
	password, err := components.GetPasswordHasher().HashPassword(input.Body.Password)
	if err != nil {
		return nil, huma.Error500InternalServerError("unable to hash password")
	}
	token, err := components.GetTokenSigner().GenerateRandToken()
	if err != nil {
		return nil, huma.Error500InternalServerError("unable to generate token")
	}
	access, err := encodeAccess(input.Body.Access)
	if err != nil {
		return nil, err
	}
	account := &accounts.User{
		Name:     normalizeName(input.Body.Name),
		Email:    email,
		Password: password,
		APIToken: token,
		Access:   access,
	}
	if err := accounts.Create(ctx, account); err != nil {
		return nil, writeError(err)
	}
	return userOutput(account)
}

func UpdateHandler(ctx context.Context, input *UpdateInput) (*UserOutput, error) {
	account, exists, err := accounts.GetByID(ctx, input.Body.ID)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("user update is unavailable")
	}
	if !exists {
		return nil, huma.Error404NotFound("user not found")
	}
	email, err := normalizeEmail(input.Body.Email)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(input.Body.Password, false); err != nil {
		return nil, err
	}
	access, err := encodeAccess(input.Body.Access)
	if err != nil {
		return nil, err
	}
	updated := *account
	updated.Name = normalizeName(input.Body.Name)
	updated.Email = email
	updated.Access = access
	if input.Body.Password != "" {
		updated.Password, err = components.GetPasswordHasher().HashPassword(input.Body.Password)
		if err != nil {
			return nil, huma.Error500InternalServerError("unable to hash password")
		}
	}
	if err := accounts.Update(ctx, &updated); err != nil {
		return nil, writeError(err)
	}
	return userOutput(&updated)
}

func publicUser(account *accounts.User) (PublicUser, error) {
	var access []string
	if err := json.Unmarshal([]byte(account.Access), &access); err != nil || access == nil {
		return PublicUser{}, errors.New("invalid access")
	}
	return PublicUser{
		ID:         account.ID,
		Name:       account.Name,
		Email:      account.Email,
		Access:     access,
		RefUserID:  account.RefUserID,
		RefChannel: account.RefChannel,
		LastLogin:  account.LastLogin,
		RegIP:      account.RegIP,
		CreatedAt:  account.CreatedAt,
		UpdatedAt:  account.UpdatedAt,
	}, nil
}

func userOutput(account *accounts.User) (*UserOutput, error) {
	user, err := publicUser(account)
	if err != nil {
		return nil, huma.Error500InternalServerError("stored user access is invalid")
	}
	return &UserOutput{Body: user}, nil
}

func normalizeName(name *string) *string {
	if name == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*name)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeEmail(email string) (string, error) {
	email, err := accounts.NormalizeEmail(email)
	if err != nil {
		return "", huma.Error400BadRequest(err.Error())
	}
	return email, nil
}

func validatePassword(password string, required bool) error {
	if password == "" && !required {
		return nil
	}
	if err := accounts.ValidateNewPassword(password); err != nil {
		return huma.Error400BadRequest(err.Error())
	}
	return nil
}

func encodeAccess(values []string) (string, error) {
	encoded, err := accounts.EncodeAccess(values)
	if err != nil {
		return "", huma.Error400BadRequest("invalid access permission")
	}
	return encoded, nil
}

func writeError(err error) error {
	var mysqlError *mysql.MySQLError
	if errors.Is(err, gorm.ErrDuplicatedKey) || errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return huma.Error409Conflict("email address is already registered")
	}
	return huma.Error503ServiceUnavailable("user write is unavailable")
}
