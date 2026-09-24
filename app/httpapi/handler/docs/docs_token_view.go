package docs

import (
	"log/slog"

	"github.com/LazyEasyDev/LZApp/components"
)

type DocsTokenView struct {
	Token             string   `json:"token"`
	AllowedOperations []string `json:"allowedOperations"`
	ShowAll           bool     `json:"showAll"`
}

// implment your view list here
func DocsTokenViewHandler(api_token string) DocsTokenView {

	signer := components.GetComponents().Security
	slog.Info("api_token", "value", api_token)
	if signer != nil && signer.Verify(api_token) == nil {
		return DocsTokenView{
			Token:             api_token,
			AllowedOperations: []string{"GET /health", "GET /setAuth", "GET /auth_check"},
			ShowAll:           true,
		}
	} else {
		return DocsTokenView{
			Token:             api_token,
			AllowedOperations: []string{"GET /auth/set", "GET /auth/check"},
			ShowAll:           false,
		}
	}

}
