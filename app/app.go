package app

import (
	"context"
	"log/slog"

	"github.com/LazyEasyDev/EasyRoutine"
	"github.com/LazyEasyDev/LZApp/app/base/dbkv"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/LazyEasyDev/LZApp/config"
)

func Run(ctx context.Context) error {
	ctx, cancelAll := context.WithCancel(ctx)
	defer func() {
		cleanupErr := components.WaitAndClose()
		if cleanupErr != nil {
			slog.Error("components cleanup failed", "error", cleanupErr)
		}
		cancelAll()
	}()

	if err := components.Init(ctx, config.GetConfig()); err != nil {
		cancelAll()
		return err
	}
	return Start(ctx, cancelAll)
}

func Start(ctx context.Context, cancelAll context.CancelFunc) error {

	// Initialize dbkv (database key-value store)////////////////////////////////
	if err := dbkv.Init(ctx, components.GetDB()); err != nil {
		cancelAll()
		return err
	}
	////////////////////////////////////////////////////////////////////////////////

	/////////////////start HTTP service if enabled////////////////////////////////
	_, err := EasyRoutine.SafeGo(
		ctx, func(taskCtx context.Context) {
			if err := startHTTPServer(taskCtx); err != nil {
				slog.Error("http service failed", "error", err)
				cancelAll()
			}
		}, func(recovered EasyRoutine.Panic, failures int) EasyRoutine.PanicDecision {
			slog.Error("http service panic", "error", recovered.Value, "stack", string(recovered.Stack))
			cancelAll()
			return EasyRoutine.NoRetry()
		},
	)
	if err != nil {
		return err
	}
	////////////////////////////////////////////////////////////////
	// add additional services here if needed
	return nil

}
