package cache

import (
	"time"

	"github.com/viccon/sturdyc"

	"github.com/KubeRocketCI/gitfusion/internal/models"
)

// NewPipelineCache creates a sturdyc cache client for pipeline lists with early refreshes enabled.
// Uses a shorter TTL (1 minute) than pull requests (2 min) due to pipeline status volatility.
func NewPipelineCache() *sturdyc.Client[models.PipelinesResponse] {
	capacity := 100
	numShards := 8
	ttl := 1 * time.Minute
	evictionPercentage := 10
	minRefreshDelay := 5 * time.Second
	maxRefreshDelay := 15 * time.Second
	synchronousRefreshDelay := 30 * time.Second
	retryBaseDelay := 2 * time.Second

	return sturdyc.New[models.PipelinesResponse](
		capacity, numShards, ttl, evictionPercentage,
		sturdyc.WithEarlyRefreshes(minRefreshDelay, maxRefreshDelay, synchronousRefreshDelay, retryBaseDelay),
	)
}
