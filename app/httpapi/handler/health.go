package handler

import (
	"context"
	"time"

	"github.com/LazyEasyDev/LZApp/app/httpapi/middleware"
	"github.com/danielgtaylor/huma/v2"
)

type HealthOutput struct {
	Body struct {
		RemoteIp       string `json:"remote_ip" example:"127.0.0.1"`
		ServerUnixTime int64  `json:"server_unix_time" example:"1680000000"`
	}
}

func HealthHandler(ctx context.Context, req *struct{}) (*HealthOutput, error) {
	response := &HealthOutput{}
	response.Body.RemoteIp = middleware.GetClientIP(ctx) // You can set this to the actual remote IP if available
	response.Body.ServerUnixTime = time.Now().Unix()

	if response.Body.ServerUnixTime%2 == 0 {
		return nil, huma.Error403Forbidden("xxxxx")
	}
	return response, nil
}
