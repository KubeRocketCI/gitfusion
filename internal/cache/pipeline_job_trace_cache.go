package cache

import (
	"time"

	"github.com/viccon/sturdyc"
)

// MaxCacheableTraceBytes caps which traces are cached; larger or truncated traces bypass the cache.
const MaxCacheableTraceBytes = 512 * 1024

// Job-trace tiers: running traces cached briefly (live), finished traces long (done).
const (
	traceLiveTTL  = 30 * time.Second
	traceDoneTTL  = 12 * time.Hour
	traceLiveSize = 20
	traceDoneSize = 80
)

type JobTrace struct {
	Content   string
	Truncated bool
}

func NewPipelineJobTraceCache() *TerminalAwareCache[JobTrace] {
	// numShards kept low so shardSize*evictionPercentage >= 1; otherwise forced eviction rounds
	// to 0 and the cap is not enforced until the TTL sweep.
	numShards := 4
	evictionPercentage := 20

	live := sturdyc.New[JobTrace](traceLiveSize, numShards, traceLiveTTL, evictionPercentage)
	done := sturdyc.New[JobTrace](traceDoneSize, numShards, traceDoneTTL, evictionPercentage)

	return NewTerminalAwareCache(live, done)
}

// terminalJobsTTL must be >= traceDoneTTL, else a finished job's trace is demoted to the live
// tier (and re-fetched) once its marker expires.
const (
	terminalJobsTTL  = 24 * time.Hour
	terminalJobsSize = 5000
)

// NewTerminalJobsCache records job IDs known terminal, so GetJobTrace (which gets only a job ID,
// no status) can decide whether a trace is immutable and long-cacheable.
func NewTerminalJobsCache() *sturdyc.Client[bool] {
	numShards := 8
	evictionPercentage := 10

	return sturdyc.New[bool](terminalJobsSize, numShards, terminalJobsTTL, evictionPercentage)
}
