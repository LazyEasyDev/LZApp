package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	appcli "github.com/LazyEasyDev/LZApp/cli"
	_ "github.com/LazyEasyDev/LZApp/local"
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
