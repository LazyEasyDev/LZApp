package lcache

import (
	"github.com/LazyEasyDev/LCache"
	"github.com/LazyEasyDev/LZApp/config"
)

func Init(cacheConfig *config.LocalCacheConfig) {
	LCache.Init(LCache.Config{MaxTTLSeconds: cacheConfig.MaxTTLSeconds})
}

func Close() {
	LCache.Close()
}
