package common

import (
	"context"

	"github.com/LazyEasyDev/LZApp/common/cache_helper"
)

func Initialize(ctx context.Context) {
	// Add initialization logic here
	cache_helper.Init(ctx)
}
