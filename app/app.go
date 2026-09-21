package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/LazyEasyDev/EasyRoutine"
	"github.com/LazyEasyDev/LZApp/app/http_server"
)

func Start(ctx context.Context, cancelAll context.CancelCauseFunc) error {

	_, err := EasyRoutine.SafeGo(
		ctx, func(taskCtx context.Context) {
			if err := http_server.Start(taskCtx); err != nil {
				cancelAll(fmt.Errorf("http service: %w", err))
			}
		}, func(recovered EasyRoutine.Panic, failures int) EasyRoutine.PanicDecision {
			slog.Error("Service panicked", "http service panic", recovered.Value, "stack", string(recovered.Stack))
			cancelAll(fmt.Errorf("http service panic: %v", recovered.Value))
			return EasyRoutine.NoRetry()
		},
	)
	if err != nil {
		err = fmt.Errorf("start http service: %w", err)
		cancelAll(err)
		return err
	}
	return nil
}
