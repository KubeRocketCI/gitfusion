package cache

import (
	"github.com/viccon/sturdyc"
)

// TerminalAwareCache splits caching into a short-TTL live tier and a long-TTL done tier; Get checks
// done first, so a finished object is never shadowed by a stale live entry. It uses Get/Set (no
// in-flight de-duplication), so Invalidate is best-effort (scan then delete).
type TerminalAwareCache[T any] struct {
	live *sturdyc.Client[T]
	done *sturdyc.Client[T]
}

func NewTerminalAwareCache[T any](live, done *sturdyc.Client[T]) *TerminalAwareCache[T] {
	return &TerminalAwareCache[T]{live: live, done: done}
}

func (c *TerminalAwareCache[T]) Get(key string) (T, bool) {
	if v, ok := c.done.Get(key); ok {
		return v, true
	}

	return c.live.Get(key)
}

func (c *TerminalAwareCache[T]) Set(key string, value T, terminal bool) {
	if terminal {
		c.done.Set(key, value)

		return
	}

	c.live.Set(key, value)
}

func (c *TerminalAwareCache[T]) Invalidate() {
	for _, key := range c.done.ScanKeys() {
		c.done.Delete(key)
	}

	for _, key := range c.live.ScanKeys() {
		c.live.Delete(key)
	}
}
