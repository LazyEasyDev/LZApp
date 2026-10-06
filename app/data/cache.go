package data

import (
	"context"
	"encoding/json"
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
	record   *T
	notFound bool
}

// GetRLCached uses Redis (R) and local cache (L) with DefaultRLCacheTTL.
// Lookups check local cache, then Redis, then load. Redis values are JSON-encoded
// on writes and JSON-decoded on reads, so T must support both operations.
// Missing records are cached as JSON null in Redis.
// Both load and GetRLCached return (record, false, nil) when found,
// (nil, true, nil) when not found, or (nil, false, err) on failure.
// forceUpdate bypasses cache reads and singleflight, then refreshes both caches.
func GetRLCached[T any](ctx context.Context, key string, cache *redis.Client, forceUpdate bool, load func(context.Context) (*T, bool, error)) (*T, bool, error) {
	return GetRLCachedWithTTL(ctx, key, cache, DefaultRLCacheTTL(), forceUpdate, load)
}

// GetRLCachedWithTTL uses Redis (R) and local cache (L) with TTLs in seconds.
// It has the same lookup and forceUpdate behavior as GetRLCached, including JSON
// encoding and decoding for Redis values and JSON null for missing records.
func GetRLCachedWithTTL[T any](ctx context.Context, key string, cache *redis.Client, ttl RLCacheTTL, forceUpdate bool, load func(context.Context) (*T, bool, error)) (*T, bool, error) {
	return getCachedWithTTL(ctx, key, cache, ttl, forceUpdate, load)
}

// GetLCached uses only local cache (L) with DefaultLCacheTTL, then calls load on
// a miss. It does not use Redis or JSON encoding, so T need not support JSON.
// Both load and GetLCached return (record, false, nil) when found,
// (nil, true, nil) when not found, or (nil, false, err) on failure.
// forceUpdate bypasses cache reads and singleflight, then refreshes local cache.
func GetLCached[T any](ctx context.Context, key string, forceUpdate bool, load func(context.Context) (*T, bool, error)) (*T, bool, error) {
	return GetLCachedWithTTL(ctx, key, DefaultLCacheTTL(), forceUpdate, load)
}

// GetLCachedWithTTL uses only local cache (L) with TTLs in seconds.
// It has the same lookup and forceUpdate behavior as GetLCached and does not
// use Redis or JSON encoding.
func GetLCachedWithTTL[T any](ctx context.Context, key string, ttl LCacheTTL, forceUpdate bool, load func(context.Context) (*T, bool, error)) (*T, bool, error) {
	return getCachedWithTTL(ctx, key, nil, RLCacheTTL{
		LocalSeconds:         ttl.LocalSeconds,
		NotFoundLocalSeconds: ttl.NotFoundLocalSeconds,
	}, forceUpdate, load)
}

func getCachedWithTTL[T any](ctx context.Context, key string, cache *redis.Client, ttl RLCacheTTL, forceUpdate bool, load func(context.Context) (*T, bool, error)) (*T, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	loadAndCache := func() (*T, bool, error) {
		record, notFound, err := load(ctx)
		if err != nil {
			return nil, false, err
		}
		if notFound {
			LCache.Set(key, (*T)(nil), ttl.NotFoundLocalSeconds)
			if cache != nil {
				cache.Set(ctx, key, "null", time.Duration(ttl.NotFoundRedisSeconds)*time.Second)
			}
			return nil, true, nil
		}
		LCache.Set(key, record, ttl.LocalSeconds)
		if cache != nil {
			if encoded, err := json.Marshal(record); err == nil {
				cache.Set(ctx, key, encoded, time.Duration(ttl.RedisSeconds)*time.Second)
			}
		}
		return record, false, nil
	}
	if forceUpdate {
		return loadAndCache()
	}
	getLocal := func() (*T, bool) {
		cached, found := LCache.Get(key)
		if !found {
			return nil, false
		}
		record, ok := cached.(*T)
		return record, ok
	}
	if record, found := getLocal(); found {
		return record, record == nil, nil
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
			return cacheLookup[T]{record: record, notFound: record == nil}, nil
		}
		if cache != nil {
			if encoded, err := cache.Get(ctx, key).Result(); err == nil {
				var record *T
				if err := json.Unmarshal([]byte(encoded), &record); err == nil {
					if record == nil {
						LCache.Set(key, (*T)(nil), ttl.NotFoundLocalSeconds)
						return cacheLookup[T]{notFound: true}, nil
					}
					LCache.Set(key, record, ttl.LocalSeconds)
					return cacheLookup[T]{record: record}, nil
				}
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		record, notFound, err := loadAndCache()
		return cacheLookup[T]{record: record, notFound: notFound}, err
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
		return cached.record, cached.notFound, nil
	}
}
