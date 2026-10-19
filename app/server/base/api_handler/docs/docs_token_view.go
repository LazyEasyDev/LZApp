package docs

import (
	"encoding/json"
	"slices"

	"github.com/LazyEasyDev/LZApp/app/users"
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
			"GET /user/config",
			"GET /user/captcha",
			"POST /user/login",
			"POST /user/register",
			"POST /user/email_code",
			"POST /user/reset_password",
			"POST /user/logout",
		},
	}
	if account == nil || account.ID == 0 {
		return view
	}
	view.Token = account.APIToken
	var access []string
	if json.Unmarshal([]byte(account.Access), &access) == nil {
		if slices.Contains(access, users.AccessUser) {
			view.AllowedOperations = append(view.AllowedOperations, "GET /user", "POST /user/ref")
		}
		view.ShowAll = slices.Contains(access, users.AccessAdmin) || slices.Contains(access, users.AccessViewAll)
		if view.ShowAll || slices.Contains(access, users.AccessAM) {
			view.AllowedOperations = append(view.AllowedOperations, "GET /am/refstatistics/list")
		}
	}
	return view
}
