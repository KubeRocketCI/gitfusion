package pipelines

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viccon/sturdyc"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

// fakeJobsProvider implements both PipelineProvider and PipelineJobsProvider and counts
// how many times the jobs/trace methods are invoked, so tests can prove caching behavior.
// Counters are mutex-guarded so concurrency tests are race-free; an optional block channel
// holds a fetch open so callers pile into a single in-flight request.
type fakeJobsProvider struct {
	mu         sync.Mutex
	listCalls  int
	traceCalls int

	jobs      []models.PipelineJob
	traceText string
	truncated bool

	block chan struct{}
}

func (f *fakeJobsProvider) TriggerPipeline(
	_ context.Context, _ string, _ string, _ []models.PipelineVariable, _ krci.GitServerSettings,
) (*models.PipelineResponse, error) {
	return nil, nil
}

func (f *fakeJobsProvider) ListPipelines(
	_ context.Context, _ string, _ krci.GitServerSettings, _ models.PipelineListOptions,
) (*models.PipelinesResponse, error) {
	return nil, nil
}

func (f *fakeJobsProvider) ListPipelineJobs(
	_ context.Context, _ string, _ int, _ krci.GitServerSettings,
) ([]models.PipelineJob, error) {
	f.mu.Lock()
	f.listCalls++
	f.mu.Unlock()

	if f.block != nil {
		<-f.block
	}

	return f.jobs, nil
}

func (f *fakeJobsProvider) GetJobTrace(
	_ context.Context, _ string, _ int, _ krci.GitServerSettings,
) (string, bool, error) {
	f.mu.Lock()
	f.traceCalls++
	f.mu.Unlock()

	if f.block != nil {
		<-f.block
	}

	return f.traceText, f.truncated, nil
}

func gitlabSettings() krci.GitServerSettings {
	return krci.GitServerSettings{GitProvider: "gitlab", GitServerName: "gs"}
}

func TestIsTerminalJobStatus(t *testing.T) {
	for _, s := range []string{"success", "failed", "canceled", "skipped"} {
		assert.True(t, isTerminalJobStatus(s), "%q should be terminal", s)
	}

	for _, s := range []string{"running", "pending", "manual", "created", ""} {
		assert.False(t, isTerminalJobStatus(s), "%q should not be terminal", s)
	}
}

func TestMultiProviderPipelineService_ListPipelineJobs_MarksTerminalJobs(t *testing.T) {
	svc := NewMultiProviderPipelineService()
	fake := &fakeJobsProvider{jobs: []models.PipelineJob{
		{Id: "5", Name: "build", Status: "success"},
		{Id: "6", Name: "test", Status: "running"},
	}}
	svc.providers["gitlab"] = fake

	_, err := svc.ListPipelineJobs(context.Background(), "proj", 7, gitlabSettings())
	require.NoError(t, err)

	_, ok := svc.terminalJobs.Get(terminalJobKey("gs", "5"))
	assert.True(t, ok, "finished job should be marked terminal for long-cached traces")

	_, ok = svc.terminalJobs.Get(terminalJobKey("gs", "6"))
	assert.False(t, ok, "running job should not be marked terminal")
}

func TestMultiProviderPipelineService_ListPipelineJobs_CachesResult(t *testing.T) {
	svc := NewMultiProviderPipelineService()
	fake := &fakeJobsProvider{jobs: []models.PipelineJob{{Id: "1", Name: "build", Status: "success"}}}
	svc.providers["gitlab"] = fake

	first, err := svc.ListPipelineJobs(context.Background(), "proj", 7, gitlabSettings())
	require.NoError(t, err)
	assert.Len(t, first, 1)

	second, err := svc.ListPipelineJobs(context.Background(), "proj", 7, gitlabSettings())
	require.NoError(t, err)
	assert.Equal(t, first, second)

	assert.Equal(t, 1, fake.listCalls, "second identical request should be served from cache")
}

func TestMultiProviderPipelineService_GetJobTrace_CachesSmallCompleteTrace(t *testing.T) {
	svc := NewMultiProviderPipelineService()
	fake := &fakeJobsProvider{traceText: "log line", truncated: false}
	svc.providers["gitlab"] = fake

	content, truncated, err := svc.GetJobTrace(context.Background(), "proj", 42, gitlabSettings())
	require.NoError(t, err)
	assert.Equal(t, "log line", content)
	assert.False(t, truncated)

	content2, truncated2, err := svc.GetJobTrace(context.Background(), "proj", 42, gitlabSettings())
	require.NoError(t, err)
	assert.Equal(t, content, content2)
	assert.Equal(t, truncated, truncated2)

	assert.Equal(t, 1, fake.traceCalls, "complete small trace should be served from cache")
}

func TestMultiProviderPipelineService_GetJobTrace_DoesNotCacheTruncatedTrace(t *testing.T) {
	svc := NewMultiProviderPipelineService()
	fake := &fakeJobsProvider{traceText: "partial", truncated: true}
	svc.providers["gitlab"] = fake

	_, _, err := svc.GetJobTrace(context.Background(), "proj", 42, gitlabSettings())
	require.NoError(t, err)

	_, _, err = svc.GetJobTrace(context.Background(), "proj", 42, gitlabSettings())
	require.NoError(t, err)

	assert.Equal(t, 2, fake.traceCalls, "truncated trace must not be cached")
}

func TestMultiProviderPipelineService_GetJobTrace_DoesNotCacheOversizedTrace(t *testing.T) {
	svc := NewMultiProviderPipelineService()
	fake := &fakeJobsProvider{
		traceText: strings.Repeat("a", cache.MaxCacheableTraceBytes+1),
		truncated: false,
	}
	svc.providers["gitlab"] = fake

	_, _, err := svc.GetJobTrace(context.Background(), "proj", 42, gitlabSettings())
	require.NoError(t, err)

	_, _, err = svc.GetJobTrace(context.Background(), "proj", 42, gitlabSettings())
	require.NoError(t, err)

	assert.Equal(t, 2, fake.traceCalls, "trace larger than the cache cap must not be cached")
}

// TestMultiProviderPipelineService_ListPipelineJobs_TerminalNotCachedLong is the regression guard
// for the retry-resurrection bug: an all-terminal job list must NOT be cached in a long immutable
// tier, because retrying a finished GitLab pipeline resurrects it with new jobs. We inject a
// clock-controlled cache, prove the terminal snapshot is served within the TTL, then advance past
// the TTL and prove the resurrected pipeline is fetched fresh.
func TestMultiProviderPipelineService_ListPipelineJobs_TerminalNotCachedLong(t *testing.T) {
	svc := NewMultiProviderPipelineService()
	fake := &fakeJobsProvider{jobs: []models.PipelineJob{{Id: "1", Name: "build", Status: "success"}}}
	svc.providers["gitlab"] = fake

	clock := sturdyc.NewTestClock(time.Now())
	ttl := time.Minute
	svc.jobsCache = sturdyc.New[[]models.PipelineJob](100, 4, ttl, 10,
		sturdyc.WithClock(clock), sturdyc.WithNoContinuousEvictions())

	first, err := svc.ListPipelineJobs(context.Background(), "proj", 7, gitlabSettings())
	require.NoError(t, err)
	assert.Len(t, first, 1)
	assert.Equal(t, 1, fake.listCalls)

	// The pipeline is retried in GitLab: it is resurrected with a new running job.
	fake.jobs = []models.PipelineJob{
		{Id: "1", Name: "build", Status: "success"},
		{Id: "2", Name: "build", Status: "running"},
	}

	within, err := svc.ListPipelineJobs(context.Background(), "proj", 7, gitlabSettings())
	require.NoError(t, err)
	assert.Len(t, within, 1, "served from cache within TTL")
	assert.Equal(t, 1, fake.listCalls)

	clock.Add(ttl + time.Second)

	after, err := svc.ListPipelineJobs(context.Background(), "proj", 7, gitlabSettings())
	require.NoError(t, err)
	assert.Len(t, after, 2, "resurrected pipeline must be re-fetched after the short TTL")
	assert.Equal(t, 2, fake.listCalls)
}

// TestMultiProviderPipelineService_ListPipelineJobs_DeduplicatesConcurrent proves a burst of
// concurrent requests for the same uncached pipeline collapses to a single provider call.
func TestMultiProviderPipelineService_ListPipelineJobs_DeduplicatesConcurrent(t *testing.T) {
	svc := NewMultiProviderPipelineService()
	fake := &fakeJobsProvider{
		jobs:  []models.PipelineJob{{Id: "1", Name: "build", Status: "running"}},
		block: make(chan struct{}),
	}
	svc.providers["gitlab"] = fake

	const n = 25

	var wg sync.WaitGroup

	wg.Add(n)

	for range n {
		go func() {
			defer wg.Done()

			_, _ = svc.ListPipelineJobs(context.Background(), "proj", 7, gitlabSettings())
		}()
	}

	// Let the goroutines pile into the in-flight call, then release the held fetch.
	time.Sleep(50 * time.Millisecond)
	close(fake.block)
	wg.Wait()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, 1, fake.listCalls, "concurrent misses must collapse to a single fetch")
}

// TestMultiProviderPipelineService_GetJobTrace_DeduplicatesConcurrent proves the same for traces,
// which de-duplicate via singleflight rather than sturdyc's GetOrFetch.
func TestMultiProviderPipelineService_GetJobTrace_DeduplicatesConcurrent(t *testing.T) {
	svc := NewMultiProviderPipelineService()
	fake := &fakeJobsProvider{traceText: "log", truncated: false, block: make(chan struct{})}
	svc.providers["gitlab"] = fake

	const n = 25

	var wg sync.WaitGroup

	wg.Add(n)

	for range n {
		go func() {
			defer wg.Done()

			_, _, _ = svc.GetJobTrace(context.Background(), "proj", 42, gitlabSettings())
		}()
	}

	time.Sleep(50 * time.Millisecond)
	close(fake.block)
	wg.Wait()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, 1, fake.traceCalls, "concurrent trace misses must collapse to a single fetch")
}
