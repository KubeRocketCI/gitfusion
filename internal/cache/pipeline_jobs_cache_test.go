package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPipelineJobsCache(t *testing.T) {
	cache := NewPipelineJobsCache()

	assert.NotNil(t, cache, "pipeline jobs cache should not be nil")

	_, ok := cache.Get("missing")
	assert.False(t, ok, "new cache should have no entries")
}
