package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPipelineCache(t *testing.T) {
	cache := NewPipelineCache()

	assert.NotNil(t, cache, "pipeline cache should not be nil")

	// Verify cache works by checking ScanKeys on an empty cache
	keys := cache.ScanKeys()
	assert.Empty(t, keys, "new cache should have no keys")
}
