package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	easyroutine "github.com/LazyEasyDev/EasyRoutine"
	"github.com/LazyEasyDev/LZApp/app/httpapi"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/LazyEasyDev/LZApp/config"
)

func Run(ctx context.Context) (runErr error) {
	ctx, cancelAll := context.WithCancelCause(ctx)
	defer cancelAll(nil)
	defer func() {
		cleanupErr := components.WaitAndClose()
		cause := context.Cause(ctx)
		if errors.Is(cause, context.Canceled) || errors.Is(cause, runErr) {
			cause = nil
		}
		runErr = errors.Join(runErr, cause, cleanupErr)
	}()

	if err := components.Init(ctx, config.GetConfig()); err != nil {
		return err
	}
	return Start(ctx, cancelAll)
}

func Start(ctx context.Context, cancelAll context.CancelCauseFunc) error {

	//start HTTP service if enabled
	if config.GetConfig().HTTP.Enabled {
		_, err := easyroutine.SafeGo(
			ctx, func(taskCtx context.Context) {
				if err := httpapi.Start(taskCtx); err != nil {
					cancelAll(fmt.Errorf("http service: %w", err))
				}
			}, func(recovered easyroutine.Panic, failures int) easyroutine.PanicDecision {
				slog.Error("Service panicked", "http service panic", recovered.Value, "stack", string(recovered.Stack))
				cancelAll(fmt.Errorf("http service panic: %v", recovered.Value))
				return easyroutine.NoRetry()
			},
		)
		if err != nil {
			err = fmt.Errorf("start http service: %w", err)
			cancelAll(err)
			return err
		}
	}

	//implement your code here
	return nil
}
