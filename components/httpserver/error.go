package httpserver

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type ErrorModel struct {
	status int

	Type   string `json:"type" format:"uri" example:"about:blank" doc:"URI identifying the error type"`
	Title  string `json:"title" example:"Bad Request" doc:"Short summary of the error type"`
	Detail string `json:"detail" example:"The request could not be processed" doc:"Explanation of this error occurrence"`
}

func (model *ErrorModel) Error() string {
	return model.Detail
}

func (model *ErrorModel) GetStatus() int {
	return model.status
}

func (model *ErrorModel) ContentType(contentType string) string {
	if contentType == "application/json" {
		return "application/problem+json"
	}
	return contentType
}

func newError(status int, detail string, _ ...error) huma.StatusError {
	return &ErrorModel{
		status: status,
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Detail: detail,
	}
}
