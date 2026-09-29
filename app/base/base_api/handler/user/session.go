package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LazyEasyDev/LZApp/app/base/base_api/middleware"
	"github.com/LazyEasyDev/LZApp/app/base/captcha"
	"github.com/LazyEasyDev/LZApp/app/base/users"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/LazyEasyDev/LZApp/config"
	"github.com/danielgtaylor/huma/v2"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const emailCodeTTL = 10 * time.Minute

type CaptchaProof struct {
	CaptchaID string `json:"captcha_id" minLength:"26" maxLength:"26"`
	Captcha   string `json:"captcha" pattern:"^[0-9]{6}$"`
}

type LoginInput struct {
	Body struct {
		CaptchaProof
		Email    string `json:"email" minLength:"3" maxLength:"254"`
		Password string `json:"password" minLength:"1" maxLength:"72"`
	}
}

type RegisterInput struct {
	Body struct {
		CaptchaProof
		Name      *string `json:"name,omitempty" maxLength:"100"`
		Email     string  `json:"email" minLength:"3" maxLength:"254"`
		Password  string  `json:"password" minLength:"1" maxLength:"72"`
		EmailCode string  `json:"email_code" pattern:"^[0-9]{6}$"`
	}
}

type ResetPasswordInput struct {
	CookieInput
	Body struct {
		CaptchaProof
		Email       string `json:"email" minLength:"3" maxLength:"254"`
		NewPassword string `json:"password" minLength:"1" maxLength:"72"`
		EmailCode   string `json:"email_code" pattern:"^[0-9]{6}$"`
	}
}

type EmailCodeInput struct {
	Body struct {
		CaptchaProof
		Email   string `json:"email" minLength:"3" maxLength:"254"`
		Purpose string `json:"purpose" enum:"register,reset_password"`
	}
}

type CaptchaOutput struct {
	Body struct {
		CaptchaID string `json:"captcha_id"`
		Image     string `json:"image"`
	}
}

type CookieInput struct {
	Cookies []*http.Cookie `json:"-"`
}

func (input *CookieInput) Resolve(ctx huma.Context) []error {
	input.Cookies = huma.ReadCookies(ctx)
	return nil
}

type UserOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      users.User
}

type MessageOutput struct {
	SetCookie     []http.Cookie `header:"Set-Cookie"`
	ClearSiteData string        `header:"Clear-Site-Data"`
	Body          struct {
		Message string `json:"message"`
	}
}

func messageOutput(message string) *MessageOutput {
	output := &MessageOutput{}
	output.Body.Message = message
	return output
}

func RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-user",
		Summary:     "Get user",
		Method:      http.MethodGet,
		Path:        "/user",
		Tags:        []string{"User"},
		Security:    []map[string][]string{{"bearerAuth": {}}, {"cookieAuth": {}}},
		Middlewares: huma.Middlewares{
			middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 60, Window: time.Minute}),
			middleware.UserAuthMiddleware(api, []string{users.ACCESS_USER}),
		},
	}, CurrentHandler)

	huma.Register(api, huma.Operation{
		OperationID: "get-user-captcha",
		Summary:     "Get user captcha",
		Method:      http.MethodGet,
		Path:        "/user/captcha",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{
			middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 60, Window: time.Minute}),
		},
	}, CaptchaHandler)

	huma.Register(api, huma.Operation{
		OperationID: "post-user-login",
		Summary:     "Post user login",
		Method:      http.MethodPost,
		Path:        "/user/login",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{
			middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 20, Window: time.Minute}),
		},
	}, LoginHandler)

	huma.Register(api, huma.Operation{
		OperationID: "post-user-register",
		Summary:     "Post user register",
		Method:      http.MethodPost,
		Path:        "/user/register",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{
			middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 10, Window: time.Minute}),
		},
	}, RegisterHandler)

	huma.Register(api, huma.Operation{
		OperationID: "post-user-reset-password",
		Summary:     "Post user reset password",
		Method:      http.MethodPost,
		Path:        "/user/reset_password",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{
			middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 10, Window: time.Minute}),
		},
	}, ResetPasswordHandler)

	huma.Register(api, huma.Operation{
		OperationID: "post-user-email-code",
		Summary:     "Post user email code",
		Method:      http.MethodPost,
		Path:        "/user/email_code",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{
			middleware.SpeedLimitMiddleware(api, middleware.SpeedLimitPolicy{Requests: 5, Window: time.Minute}),
		},
	}, EmailCodeHandler)

	huma.Register(api, huma.Operation{
		OperationID: "post-user-logout",
		Summary:     "Post user logout",
		Method:      http.MethodPost,
		Path:        "/user/logout",
		Tags:        []string{"User"},
	}, LogoutHandler)
}

func tokenCookie(token string) (http.Cookie, error) {
	settings := config.GetConfig().HTTP
	if settings == nil || settings.APITokenCookieName == "" {
		return http.Cookie{}, huma.Error500InternalServerError("cookie is not configured")
	}
	cookie := http.Cookie{
		Name:     settings.APITokenCookieName,
		Value:    token,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	if token == "" {
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0)
	}
	if err := cookie.Valid(); err != nil {
		return http.Cookie{}, huma.Error500InternalServerError("invalid cookie configuration")
	}
	return cookie, nil
}

func publicUser(account *users.User) users.User {
	result := *account
	result.Password = ""
	return result
}

func signedInOutput(account *users.User) (*UserOutput, error) {
	if !account.HaveAllAccess([]string{users.ACCESS_USER}) {
		return nil, huma.Error403Forbidden("user access required")
	}
	cookie, err := tokenCookie(account.ApiToken)
	if err != nil {
		return nil, err
	}
	return &UserOutput{SetCookie: []http.Cookie{cookie}, Body: publicUser(account)}, nil
}

func CurrentHandler(ctx context.Context, _ *struct{}) (*UserOutput, error) {
	account, authenticated := middleware.GetAuthUser(ctx)
	if !authenticated {
		return nil, huma.Error401Unauthorized("login required")
	}
	return &UserOutput{Body: publicUser(account)}, nil
}

func LogoutHandler(_ context.Context, input *CookieInput) (*MessageOutput, error) {
	cookie, err := tokenCookie("")
	if err != nil {
		return nil, err
	}
	output := messageOutput("Logged out")
	output.ClearSiteData = `"cookies"`
	output.SetCookie = []http.Cookie{cookie}
	seen := map[string]bool{cookie.Name: true}
	for _, existing := range input.Cookies {
		if seen[existing.Name] {
			continue
		}
		seen[existing.Name] = true
		expired := cookie
		expired.Name = existing.Name
		output.SetCookie = append(output.SetCookie, expired)
	}
	return output, nil
}

func CaptchaHandler(_ context.Context, _ *struct{}) (*CaptchaOutput, error) {
	id, image, err := captcha.Generate(captcha.Config{})
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("captcha is unavailable")
	}
	output := &CaptchaOutput{}
	output.Body.CaptchaID = id
	output.Body.Image = "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)
	return output, nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return "", huma.Error400BadRequest("invalid email address")
	}
	return email, nil
}

func verifyCaptcha(proof CaptchaProof) error {
	if !captcha.Verify(proof.CaptchaID, proof.Captcha) {
		return huma.Error400BadRequest("invalid or expired captcha")
	}
	return nil
}

func validateNewPassword(password string) error {
	if utf8.RuneCountInString(password) < 8 || len(password) > 72 {
		return huma.Error400BadRequest("password must contain at least 8 characters and at most 72 bytes")
	}
	return nil
}

func LoginHandler(ctx context.Context, input *LoginInput) (*UserOutput, error) {
	if err := verifyCaptcha(input.Body.CaptchaProof); err != nil {
		return nil, err
	}
	email, err := normalizeEmail(input.Body.Email)
	if err != nil {
		return nil, err
	}
	account, err := users.GetByEmail(ctx, email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, huma.Error401Unauthorized("invalid email or password")
	}
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("login is unavailable")
	}
	valid, err := components.GetPasswordHasher().VerifyPassword(input.Body.Password, account.Password)
	if err != nil || !valid {
		return nil, huma.Error401Unauthorized("invalid email or password")
	}
	return signedInOutput(account)
}

func RegisterHandler(ctx context.Context, input *RegisterInput) (*UserOutput, error) {
	if err := verifyCaptcha(input.Body.CaptchaProof); err != nil {
		return nil, err
	}
	email, err := normalizeEmail(input.Body.Email)
	if err != nil {
		return nil, err
	}
	if err := validateNewPassword(input.Body.Password); err != nil {
		return nil, err
	}
	if err := verifyEmailCode(ctx, email, "register", input.Body.EmailCode); err != nil {
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
	access, _ := json.Marshal([]string{users.ACCESS_USER})
	account := &users.User{Name: input.Body.Name, Email: email, Password: password, ApiToken: token, Access: string(access)}
	if err := users.Create(ctx, account); err != nil {
		return nil, huma.Error503ServiceUnavailable(err.Error())
	}
	return signedInOutput(account)
}

func ResetPasswordHandler(ctx context.Context, input *ResetPasswordInput) (*MessageOutput, error) {
	if err := verifyCaptcha(input.Body.CaptchaProof); err != nil {
		return nil, err
	}
	email, err := normalizeEmail(input.Body.Email)
	if err != nil {
		return nil, err
	}
	if err := validateNewPassword(input.Body.NewPassword); err != nil {
		return nil, err
	}
	if err := verifyEmailCode(ctx, email, "reset_password", input.Body.EmailCode); err != nil {
		return nil, err
	}
	account, err := users.GetByEmail(ctx, email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, huma.Error400BadRequest("unable to reset password")
	}
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("password reset is unavailable")
	}
	password, err := components.GetPasswordHasher().HashPassword(input.Body.NewPassword)
	if err != nil {
		return nil, huma.Error500InternalServerError("unable to hash password")
	}
	token, err := components.GetTokenSigner().GenerateRandToken()
	if err != nil {
		return nil, huma.Error500InternalServerError("unable to generate token")
	}
	updated := *account
	updated.Password = password
	updated.ApiToken = token
	if err := users.Update(ctx, &updated); err != nil {
		return nil, huma.Error503ServiceUnavailable(err.Error())
	}
	output, err := LogoutHandler(ctx, &input.CookieInput)
	if err != nil {
		return nil, err
	}
	output.Body.Message = "Password reset; sign in with your new password"
	return output, nil
}

func emailCodeKey(email, purpose string) string {
	return fmt.Sprintf("user-email-code:%s:%x", purpose, sha256.Sum256([]byte(email)))
}

func EmailCodeHandler(ctx context.Context, input *EmailCodeInput) (*MessageOutput, error) {
	if err := verifyCaptcha(input.Body.CaptchaProof); err != nil {
		return nil, err
	}
	email, err := normalizeEmail(input.Body.Email)
	if err != nil {
		return nil, err
	}
	if input.Body.Purpose != "register" && input.Body.Purpose != "reset_password" {
		return nil, huma.Error400BadRequest("invalid verification purpose")
	}
	client := components.GetRedis()
	if client == nil || components.GetEmailSender() == nil {
		return nil, huma.Error503ServiceUnavailable("email verification is unavailable")
	}
	cooldownKey := emailCodeKey(email, "cooldown")
	allowed, err := client.SetNX(ctx, cooldownKey, "1", 30*time.Second).Result()
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("email verification is unavailable")
	}
	if !allowed {
		return nil, huma.Error429TooManyRequests("wait 30 seconds before requesting another email")
	}
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return nil, huma.Error500InternalServerError("unable to generate verification code")
	}
	code := fmt.Sprintf("%06d", number.Int64())
	key := emailCodeKey(email, input.Body.Purpose)
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(code)))
	if err := client.Set(ctx, key, digest, emailCodeTTL).Err(); err != nil {
		return nil, huma.Error503ServiceUnavailable("email verification is unavailable")
	}
	action := "registration"
	if input.Body.Purpose == "reset_password" {
		action = "password reset"
	}
	body := fmt.Sprintf("Your %s verification code is %s. It expires in 10 minutes. If you did not request this code, ignore this email.", action, code)
	if err := components.GetEmailSender().Send(ctx, email, action+" code", body); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_ = client.Del(cleanupCtx, key).Err()
		return nil, huma.Error503ServiceUnavailable("unable to send verification email")
	}
	return messageOutput("Verification code sent"), nil
}

func verifyEmailCode(ctx context.Context, email, purpose, code string) error {
	client := components.GetRedis()
	if client == nil {
		return huma.Error503ServiceUnavailable("email verification is unavailable")
	}
	stored, err := client.GetDel(ctx, emailCodeKey(email, purpose)).Result()
	if err != nil && !errors.Is(err, goredis.Nil) {
		return huma.Error503ServiceUnavailable("email verification is unavailable")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(code)))
	if err != nil || len(code) != 6 || subtle.ConstantTimeCompare([]byte(stored), []byte(digest)) != 1 {
		return huma.Error400BadRequest("invalid or expired email code; request a new code")
	}
	return nil
}
