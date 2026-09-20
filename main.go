package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	appcli "github.com/LazyEasyDev/LZApp/cli"
)

func main() {

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	/// Run the LZApp CLI with the provided context and arguments
	if err := appcli.Run(ctx, os.Args); err != nil {
		os.Exit(1)
	}
}
