package cache_helper

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/LazyEasyDev/LCache"
	"github.com/LazyEasyDev/LZApp/components/redis"
	"golang.org/x/sync/singleflight"
)

type LCacheTTL struct {
	LocalSeconds         int64
	NotFoundLocalSeconds int64
}

type RLCacheTTL struct {
	LocalSeconds         int64
	RedisSeconds         int64
	NotFoundLocalSeconds int64
	NotFoundRedisSeconds int64
}

const localCacheDefaultSeconds = 15
const redisCacheDefaultSeconds = 1800
const notFoundLocalCacheDefaultSeconds = 15
const notFoundRedisCacheDefaultSeconds = 60

func DefaultLCacheTTL() LCacheTTL {
	return LCacheTTL{
		LocalSeconds:         localCacheDefaultSeconds,
		NotFoundLocalSeconds: notFoundLocalCacheDefaultSeconds,
	}
}

func DefaultRLCacheTTL() RLCacheTTL {
	return RLCacheTTL{
		LocalSeconds:         localCacheDefaultSeconds,
		RedisSeconds:         redisCacheDefaultSeconds,
		NotFoundLocalSeconds: notFoundLocalCacheDefaultSeconds,
		NotFoundRedisSeconds: notFoundRedisCacheDefaultSeconds,
	}
}

var redisLocalLookups singleflight.Group
var localLookups singleflight.Group

type cacheLookup[T any] struct {
	record *T
	exists bool
}

// GetRLCached uses Redis (R) and local cache (L) with DefaultRLCacheTTL.
// item must be non-nil and supplies the key for both caches and the local tag.
// Cache writes update item.Val. Use a fresh item per call and do not mutate it
// after passing it in; canceled calls may continue using it in the background.
// Lookups check local cache, then Redis, then load. Redis values are JSON-encoded
// on writes and JSON-decoded on reads, so T must support both operations.
// Missing records are cached as JSON null in Redis.
// Both load and GetRLCached return (record, true, nil) when found,
// (nil, false, nil) when not found, or (nil, false, err) on failure.
// forceUpdate bypasses cache reads and singleflight, then refreshes both caches.
// Before loading, existing entries get a 5-second local TTL and a 60-second
// Redis TTL. Missing entries are allowed; Redis expiry errors abort the refresh.
func GetRLCached[T any](ctx context.Context, item *CacheItem, cache *redis.Client, forceUpdate bool, load func(context.Context) (*T, bool, error)) (*T, bool, error) {
	return GetRLCachedWithTTL(ctx, item, cache, DefaultRLCacheTTL(), forceUpdate, load)
}

// GetRLCachedWithTTL uses Redis (R) and local cache (L) with TTLs in seconds.
// It has the same lookup and forceUpdate behavior as GetRLCached, including JSON
// encoding and decoding for Redis values and JSON null for missing records.
func GetRLCachedWithTTL[T any](ctx context.Context, item *CacheItem, cache *redis.Client, ttl RLCacheTTL, forceUpdate bool, load func(context.Context) (*T, bool, error)) (*T, bool, error) {
	return getCachedWithTTL(ctx, item, cache, ttl, forceUpdate, load)
}

// GetLCached uses only local cache (L) with DefaultLCacheTTL, then calls load on
// a miss. It does not use Redis or JSON encoding, so T need not support JSON.
// item must be non-nil and supplies the cache key and statistics tag.
// Cache writes update item.Val. Use a fresh item per call and do not mutate it
// after passing it in; canceled calls may continue using it in the background.
// Both load and GetLCached return (record, true, nil) when found,
// (nil, false, nil) when not found, or (nil, false, err) on failure.
// forceUpdate bypasses cache reads and singleflight, then refreshes local cache.
// Before loading, any existing local entry gets a 5-second TTL.
func GetLCached[T any](ctx context.Context, item *CacheItem, forceUpdate bool, load func(context.Context) (*T, bool, error)) (*T, bool, error) {
	return GetLCachedWithTTL(ctx, item, DefaultLCacheTTL(), forceUpdate, load)
}

// GetLCachedWithTTL uses only local cache (L) with TTLs in seconds.
// It has the same lookup and forceUpdate behavior as GetLCached and does not
// use Redis or JSON encoding.
func GetLCachedWithTTL[T any](ctx context.Context, item *CacheItem, ttl LCacheTTL, forceUpdate bool, load func(context.Context) (*T, bool, error)) (*T, bool, error) {
	return getCachedWithTTL(ctx, item, nil, RLCacheTTL{
		LocalSeconds:         ttl.LocalSeconds,
		NotFoundLocalSeconds: ttl.NotFoundLocalSeconds,
	}, forceUpdate, load)
}

func getCachedWithTTL[T any](ctx context.Context, item *CacheItem, cache *redis.Client, ttl RLCacheTTL, forceUpdate bool, load func(context.Context) (*T, bool, error)) (*T, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if item == nil {
		return nil, false, errors.New("cache item is required")
	}
	key := item.GetKey()
	loadAndCache := func() (*T, bool, error) {
		record, exists, err := load(ctx)
		if err != nil {
			return nil, false, err
		}
		if !exists {
			item.Val((*T)(nil)).Set(ttl.NotFoundLocalSeconds)
			if cache != nil {
				cache.Set(ctx, key, "null", time.Duration(ttl.NotFoundRedisSeconds)*time.Second)
			}
			return nil, false, nil
		}
		item.Val(record).Set(ttl.LocalSeconds)
		if cache != nil {
			if encoded, err := json.Marshal(record); err != nil {
				slog.Error("failed to marshal cache record", "error", err)
			} else {
				cache.Set(ctx, key, encoded, time.Duration(ttl.RedisSeconds)*time.Second)
			}
		}
		return record, true, nil
	}
	if forceUpdate {
		LCache.Touch(key, 5)
		if cache != nil {
			if err := cache.Expire(ctx, key, 60*time.Second).Err(); err != nil {
				return nil, false, err
			}
		}
		return loadAndCache()
	}
	getLocal := func() (*T, bool) {
		cached := LCache.Get(key)
		if !cached.Found() {
			return nil, false
		}
		record, ok := cached.Val().(*T)
		return record, ok
	}
	if record, found := getLocal(); found {
		return record, record != nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	lookups := &localLookups
	if cache != nil {
		lookups = &redisLocalLookups
	}
	result := lookups.DoChan(key, func() (any, error) {
		if record, found := getLocal(); found {
			return cacheLookup[T]{record: record, exists: record != nil}, nil
		}
		if cache != nil {
			if encoded, err := cache.Get(ctx, key).Result(); err == nil {
				var record *T
				if err := json.Unmarshal([]byte(encoded), &record); err != nil {
					slog.Error("failed to unmarshal cache record", "error", err)
				} else {
					if record == nil {
						item.Val((*T)(nil)).Set(ttl.NotFoundLocalSeconds)
						return cacheLookup[T]{}, nil
					}
					item.Val(record).Set(ttl.LocalSeconds)
					return cacheLookup[T]{record: record, exists: true}, nil
				}
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		record, exists, err := loadAndCache()
		return cacheLookup[T]{record: record, exists: exists}, err
	})
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	case lookup := <-result:
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if lookup.Err != nil {
			return nil, false, lookup.Err
		}
		cached := lookup.Val.(cacheLookup[T])
		return cached.record, cached.exists, nil
	}
}
