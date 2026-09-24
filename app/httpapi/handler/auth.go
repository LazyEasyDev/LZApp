package handler

import (
	"context"
	"time"
)

type AuthOutput struct {
	Body struct {
		Verified       bool  `json:"verified" example:"true"`
		ServerUnixTime int64 `json:"server_unix_time" example:"1680000000"`
	}
}

func AuthHandler(ctx context.Context, _ *struct{}) (*AuthOutput, error) {
	response := &AuthOutput{}
	response.Body.Verified = true
	response.Body.ServerUnixTime = time.Now().Unix()
	return response, nil
}
