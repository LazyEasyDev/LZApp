package httpserver

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type ErrorModel struct {
	Type   string `json:"type" format:"uri" example:"about:blank" doc:"URI identifying the error type"`
	Title  string `json:"title" example:"Bad Request" doc:"Short summary of the error type"`
	Status int    `json:"status" example:"400" doc:"400|401|404...,HTTP status code"`
	Detail string `json:"detail" example:"The request could not be processed" doc:"Explanation of this error occurrence"`
}

func (model *ErrorModel) Error() string {
	return model.Detail
}

func (model *ErrorModel) GetStatus() int {
	return model.Status
}

func (model *ErrorModel) ContentType(contentType string) string {
	if contentType == "application/json" {
		return "application/problem+json"
	}
	return contentType
}

func newError(status int, detail string, _ ...error) huma.StatusError {
	return &ErrorModel{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	}
}
