package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPullRequestCache(t *testing.T) {
	cache := NewPullRequestCache()

	assert.NotNil(t, cache, "pull request cache should not be nil")

	// Verify cache works by checking ScanKeys on an empty cache
	keys := cache.ScanKeys()
	assert.Empty(t, keys, "new cache should have no keys")
}
