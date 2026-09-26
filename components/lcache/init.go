package lcache

import (
	"github.com/LazyEasyDev/LCache"
	"github.com/LazyEasyDev/LZApp/config/lcache_config"
)

func Init(cacheConfig *lcache_config.LocalCacheConfig) {
	LCache.Init(LCache.Config{MaxTTLSeconds: cacheConfig.MaxTTLSeconds})
}

func Close() {
	LCache.Close()
}
