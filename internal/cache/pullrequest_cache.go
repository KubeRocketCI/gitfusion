package cache

import (
	"time"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/viccon/sturdyc"
)

// NewPullRequestCache creates a sturdyc cache client for pull request lists with early refreshes enabled.
func NewPullRequestCache() *sturdyc.Client[models.PullRequestsResponse] {
	capacity := 100
	numShards := 8
	ttl := 2 * time.Minute
	evictionPercentage := 10
	minRefreshDelay := 10 * time.Second
	maxRefreshDelay := 30 * time.Second
	synchronousRefreshDelay := 60 * time.Second
	retryBaseDelay := 2 * time.Second

	return sturdyc.New[models.PullRequestsResponse](
		capacity, numShards, ttl, evictionPercentage,
		sturdyc.WithEarlyRefreshes(minRefreshDelay, maxRefreshDelay, synchronousRefreshDelay, retryBaseDelay),
	)
}
