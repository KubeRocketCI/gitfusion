package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPipelineJobTraceCache(t *testing.T) {
	cache := NewPipelineJobTraceCache()

	assert.NotNil(t, cache, "pipeline job trace cache should not be nil")

	_, ok := cache.Get("missing")
	assert.False(t, ok, "new cache should have no entries")
}

func TestPipelineJobTraceCache_SetGetRoundTrip(t *testing.T) {
	cache := NewPipelineJobTraceCache()

	cache.Set("key", JobTrace{Content: "hello", Truncated: false}, true)

	got, ok := cache.Get("key")
	assert.True(t, ok, "value should be present after Set")
	assert.Equal(t, "hello", got.Content)
	assert.False(t, got.Truncated)
}

func TestNewTerminalJobsCache(t *testing.T) {
	cache := NewTerminalJobsCache()

	assert.NotNil(t, cache, "terminal jobs cache should not be nil")
	assert.Empty(t, cache.ScanKeys(), "new cache should have no keys")
}
