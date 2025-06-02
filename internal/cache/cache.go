package cache

import (
	"time"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/viccon/sturdyc"
)

// newRepositoryCache creates a sturdyc cache client for repository lists with early refreshes enabled.
// Maximum number of entries in the cache. Exceeding this number triggers eviction.
func NewRepositoryCache() *sturdyc.Client[[]models.Repository] {
	// Maximum number of entries in the cache. Exceeding this number triggers eviction.
	capacity := 100
	// Number of shards to use. More shards reduce write lock contention.
	numShards := 8
	// Time-to-live for cache entries. Controls how long items stay in cache.
	ttl := 2 * time.Minute
	// Percentage of entries to evict when the cache reaches capacity.
	evictionPercentage := 10
	// Minimum delay before a background refresh is triggered for a frequently accessed key.
	minRefreshDelay := 10 * time.Second
	// Maximum delay before a background refresh is triggered for a frequently accessed key.
	maxRefreshDelay := 30 * time.Second
	// If a cache entry is older than this, a synchronous refresh is performed (user waits).
	synchronousRefreshDelay := 60 * time.Second
	// Base delay for exponential backoff when background refreshes fail.
	retryBaseDelay := 2 * time.Second

	return sturdyc.New[[]models.Repository](
		capacity, numShards, ttl, evictionPercentage,
		sturdyc.WithEarlyRefreshes(minRefreshDelay, maxRefreshDelay, synchronousRefreshDelay, retryBaseDelay),
	)
}

// newOrganizationCache creates a sturdyc cache client for organization lists with early refreshes enabled.
func NewOrganizationCache() *sturdyc.Client[[]models.Organization] {
	// Maximum number of entries in the cache. Exceeding this number triggers eviction.
	capacity := 100
	// Number of shards to use. More shards reduce write lock contention.
	numShards := 8
	// Time-to-live for cache entries. Controls how long items stay in cache.
	ttl := 30 * time.Minute
	// Percentage of entries to evict when the cache reaches capacity.
	evictionPercentage := 10
	// Minimum delay before a background refresh is triggered for a frequently accessed key.
	minRefreshDelay := 2 * time.Minute
	// Maximum delay before a background refresh is triggered for a frequently accessed key.
	maxRefreshDelay := 5 * time.Minute
	// If a cache entry is older than this, a synchronous refresh is performed (user waits).
	synchronousRefreshDelay := ttl
	// Base delay for exponential backoff when background refreshes fail.
	retryBaseDelay := 30 * time.Second

	return sturdyc.New[[]models.Organization](
		capacity, numShards, ttl, evictionPercentage,
		sturdyc.WithEarlyRefreshes(minRefreshDelay, maxRefreshDelay, synchronousRefreshDelay, retryBaseDelay),
	)
}
