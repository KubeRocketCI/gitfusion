package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/viccon/sturdyc"
)

func newStringTier() *sturdyc.Client[string] {
	return sturdyc.New[string](10, 2, time.Minute, 10)
}

func TestTerminalAwareCache_PrefersDoneTier(t *testing.T) {
	c := NewTerminalAwareCache(newStringTier(), newStringTier())

	// A non-terminal value lands in the live tier and is readable.
	c.Set("live-only", "live", false)
	got, ok := c.Get("live-only")
	assert.True(t, ok)
	assert.Equal(t, "live", got)

	// When the same key exists in both tiers, the done (immutable) value wins.
	c.Set("both", "live", false)
	c.Set("both", "done", true)
	got, ok = c.Get("both")
	assert.True(t, ok)
	assert.Equal(t, "done", got)

	_, ok = c.Get("missing")
	assert.False(t, ok)
}

func TestTerminalAwareCache_InvalidateClearsBothTiers(t *testing.T) {
	c := NewTerminalAwareCache(newStringTier(), newStringTier())

	c.Set("live", "v", false)
	c.Set("done", "v", true)

	c.Invalidate()

	_, ok := c.Get("live")
	assert.False(t, ok, "live tier should be cleared")

	_, ok = c.Get("done")
	assert.False(t, ok, "done tier should be cleared")
}
