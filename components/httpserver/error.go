package httpserver

import (
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

type ErrorModel struct {
	status int

	Type   string            `json:"type" format:"uri" example:"about:blank" doc:"URI identifying the error type"`
	Title  string            `json:"title" example:"Bad Request" doc:"Short summary of the error type"`
	Detail string            `json:"detail" example:"The request could not be processed" doc:"Explanation of this error occurrence"`
	Errors []ValidationError `json:"errors,omitempty" doc:"Individual request validation errors, without rejected values"`
}

type ValidationError struct {
	Message  string `json:"message" doc:"Validation rule that failed"`
	Location string `json:"location,omitempty" doc:"Request field that failed validation"`
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

func newError(status int, detail string, errs ...error) huma.StatusError {
	model := &ErrorModel{
		status: status,
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Detail: detail,
	}
	if status == http.StatusBadRequest || status == http.StatusUnprocessableEntity {
		for _, err := range errs {
			if field, ok := err.(*huma.ErrorDetail); ok && field != nil {
				model.Errors = append(model.Errors, ValidationError{
					Message:  validationMessage(field.Message),
					Location: field.Location,
				})
			}
		}
	}
	return model
}

func validationMessage(message string) string {
	// Parser errors can embed the rejected input in their explanations.
	switch {
	case strings.HasPrefix(message, "strconv."), strings.HasPrefix(message, "parsing time "):
		return "invalid value"
	case strings.HasPrefix(message, "expected duration:"),
		strings.HasPrefix(message, "expected string to be "),
		strings.HasPrefix(message, "invalid JSON:"):
		rule, _, _ := strings.Cut(message, ": ")
		return rule
	default:
		return message
	}
}
