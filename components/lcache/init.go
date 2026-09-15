package lcache

import (
	cachelib "github.com/LazyEasyDev/LCache"
	"github.com/LazyEasyDev/LZApp/config"
)

func Init(cacheConfig *config.LocalCacheConfig) *cachelib.Cache {
	return cachelib.New(cachelib.Config{MaxTTLSeconds: cacheConfig.MaxTTLSeconds})
}
