package app

import (
	"context"
	"log/slog"

	"github.com/LazyEasyDev/EasyRoutine"
	"github.com/LazyEasyDev/LZApp/src/app/http_service"
)

func Start(ctx context.Context) error {
	//start the service in a safe goroutine
	_, err := EasyRoutine.SafeGo(
		ctx, func(ctx context.Context) {
			// start the HTTP service
			http_service.Start(ctx)
		}, func(recovered EasyRoutine.Panic, failures int) EasyRoutine.PanicDecision {
			slog.Error(string(recovered.Stack))
			return EasyRoutine.NoRetry()
		},
	)

	return err
}
