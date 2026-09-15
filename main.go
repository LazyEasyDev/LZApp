package main

import (
	"context"
	"fmt"
	"os"

	appcli "github.com/LazyEasyDev/LZApp/cli"
)

func main() {
	if err := appcli.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
