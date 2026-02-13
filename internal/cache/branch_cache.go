package cache

import (
	"time"

	"github.com/viccon/sturdyc"

	"github.com/KubeRocketCI/gitfusion/internal/models"
)

// NewBranchCache creates a sturdyc cache client for branch lists with early refreshes enabled.
func NewBranchCache() *sturdyc.Client[[]models.Branch] {
	capacity := 100
	numShards := 8
	ttl := 5 * time.Minute
	evictionPercentage := 10
	minRefreshDelay := 10 * time.Second
	maxRefreshDelay := 30 * time.Second
	synchronousRefreshDelay := 60 * time.Second
	retryBaseDelay := 2 * time.Second

	return sturdyc.New[[]models.Branch](
		capacity, numShards, ttl, evictionPercentage,
		sturdyc.WithEarlyRefreshes(minRefreshDelay, maxRefreshDelay, synchronousRefreshDelay, retryBaseDelay),
	)
}
