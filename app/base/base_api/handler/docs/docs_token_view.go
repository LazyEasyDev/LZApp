package docs

import (
	"encoding/json"
	"slices"

	"github.com/LazyEasyDev/LZApp/app/base/users"
)

type DocsTokenView struct {
	Token             string   `json:"token"`
	AllowedOperations []string `json:"allowedOperations"`
	ShowAll           bool     `json:"showAll"`
}

func DocsTokenViewHandler(account *users.User) DocsTokenView {
	view := DocsTokenView{
		AllowedOperations: []string{
			"GET /health",
			"GET /user/captcha",
			"POST /user/login",
			"POST /user/register",
			"POST /user/email_code",
			"POST /user/reset_password",
		},
	}
	if account == nil || account.ID == 0 {
		return view
	}
	view.Token = account.ApiToken
	view.AllowedOperations = append(view.AllowedOperations, "GET /user", "GET /auth/check", "POST /user/logout")
	var access []string
	if json.Unmarshal([]byte(account.Access), &access) == nil {
		view.ShowAll = slices.Contains(access, users.ACCESS_ADMIN) || slices.Contains(access, users.ACCESS_VIEWALL)
	}
	return view
}
