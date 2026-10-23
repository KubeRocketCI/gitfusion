package pipelines

import (
	"context"
	"errors"
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
	"github.com/KubeRocketCI/gitfusion/pkg/pointer"
)

// fakeJobsProvider implements both PipelineProvider and PipelineJobsProvider and counts
// how many times the pipelines/jobs/trace methods are invoked, so tests can prove caching behavior.
// Counters are mutex-guarded so concurrency tests are race-free; an optional block channel
// holds a fetch open so callers pile into a single in-flight request.
type fakeJobsProvider struct {
	mu             sync.Mutex
	pipelinesCalls int
	jobsCalls      int
	traceCalls     int

	// lastQuery is the query the last ListPipelines call received.
	lastQuery pipelinesQuery

	pipelines    models.PipelinesResponse
	pipelinesErr error
	jobs         []models.PipelineJob
	traceText    string
	truncated    bool

	block chan struct{}
}

func (f *fakeJobsProvider) ListPipelines(
	_ context.Context, project string, settings krci.GitServerSettings, opts models.PipelineListOptions,
) (*models.PipelinesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pipelinesCalls++
	f.lastQuery = pipelinesQuery{project: project, settings: settings, opts: opts}

	if f.pipelinesErr != nil {
		return nil, f.pipelinesErr
	}

	resp := f.pipelines

	return &resp, nil
}

func (f *fakeJobsProvider) ListPipelineJobs(
	_ context.Context, _ string, _ int, _ krci.GitServerSettings,
) ([]models.PipelineJob, error) {
	f.mu.Lock()
	f.jobsCalls++
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

// pipelinesQuery holds the arguments of one ListPipelines call.
type pipelinesQuery struct {
	project  string
	settings krci.GitServerSettings
	opts     models.PipelineListOptions
}

func newPipelinesQuery() pipelinesQuery {
	return pipelinesQuery{
		project: "proj",
		settings: krci.GitServerSettings{
			Url:           "https://gitlab.example.com",
			Token:         "t0ken",
			GitProvider:   "gitlab",
			GitServerName: "gs",
		},
		opts: models.PipelineListOptions{Page: 1, PerPage: 20},
	}
}

func (q pipelinesQuery) list(svc *MultiProviderPipelineService) (*models.PipelinesResponse, error) {
	return svc.ListPipelines(context.Background(), q.project, q.settings, q.opts)
}

func TestIsTerminalJobStatus(t *testing.T) {
	for _, s := range []string{"success", "failed", "canceled", "skipped"} {
		assert.True(t, isTerminalJobStatus(s), "%q should be terminal", s)
	}

	for _, s := range []string{"running", "pending", "manual", "created", ""} {
		assert.False(t, isTerminalJobStatus(s), "%q should not be terminal", s)
	}
}

func TestMultiProviderPipelineService_ListPipelines_CachesResult(t *testing.T) {
	svc := NewMultiProviderPipelineService()
	fake := &fakeJobsProvider{pipelines: models.PipelinesResponse{
		Data:       []models.Pipeline{{Id: "101", Status: "success"}},
		Pagination: models.Pagination{Total: 1},
	}}
	svc.providers["gitlab"] = fake

	first, err := newPipelinesQuery().list(svc)
	require.NoError(t, err)
	assert.Equal(t, fake.pipelines, *first)

	second, err := newPipelinesQuery().list(svc)
	require.NoError(t, err)
	assert.Equal(t, fake.pipelines, *second)

	assert.Equal(t, 1, fake.pipelinesCalls, "second identical request should be served from cache")
}

func TestMultiProviderPipelineService_ListPipelines_CachesEachQueryFieldSeparately(t *testing.T) {
	tests := []struct {
		name   string
		change func(q *pipelinesQuery)
	}{
		{name: "ref", change: func(q *pipelinesQuery) { q.opts.Ref = pointer.To("main") }},
		{name: "status", change: func(q *pipelinesQuery) { q.opts.Status = pointer.To("failed") }},
		{name: "page", change: func(q *pipelinesQuery) { q.opts.Page = 2 }},
		{name: "per page", change: func(q *pipelinesQuery) { q.opts.PerPage = 50 }},
		{name: "project", change: func(q *pipelinesQuery) { q.project = "other" }},
		{name: "git server", change: func(q *pipelinesQuery) { q.settings.GitServerName = "other-gs" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewMultiProviderPipelineService()
			fake := &fakeJobsProvider{}
			svc.providers["gitlab"] = fake

			_, err := newPipelinesQuery().list(svc)
			require.NoError(t, err)

			changed := newPipelinesQuery()
			tt.change(&changed)

			_, err = changed.list(svc)
			require.NoError(t, err)
			assert.Equal(t, 2, fake.pipelinesCalls, "a query differing in %s must reach the provider", tt.name)
			assert.Equal(t, changed, fake.lastQuery, "the provider must receive the changed query")

			again := newPipelinesQuery()
			tt.change(&again)

			_, err = again.list(svc)
			require.NoError(t, err)
			assert.Equal(t, 2, fake.pipelinesCalls, "an equal query built anew should be served from cache")
		})
	}
}

func TestMultiProviderPipelineService_ListPipelines_DoesNotCacheError(t *testing.T) {
	svc := NewMultiProviderPipelineService()
	fetchErr := errors.New("gitlab unavailable")
	fake := &fakeJobsProvider{
		pipelines:    models.PipelinesResponse{Data: []models.Pipeline{{Id: "101"}}},
		pipelinesErr: fetchErr,
	}
	svc.providers["gitlab"] = fake

	failed, err := newPipelinesQuery().list(svc)
	require.ErrorIs(t, err, fetchErr)
	assert.Nil(t, failed)

	fake.pipelinesErr = nil

	recovered, err := newPipelinesQuery().list(svc)
	require.NoError(t, err)
	assert.Equal(t, fake.pipelines, *recovered)
	assert.Equal(t, 2, fake.pipelinesCalls, "a failed fetch must not be cached")
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

	assert.Equal(t, 1, fake.jobsCalls, "second identical request should be served from cache")
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
	assert.Equal(t, 1, fake.jobsCalls)

	// The pipeline is retried in GitLab: it is resurrected with a new running job.
	fake.jobs = []models.PipelineJob{
		{Id: "1", Name: "build", Status: "success"},
		{Id: "2", Name: "build", Status: "running"},
	}

	within, err := svc.ListPipelineJobs(context.Background(), "proj", 7, gitlabSettings())
	require.NoError(t, err)
	assert.Len(t, within, 1, "served from cache within TTL")
	assert.Equal(t, 1, fake.jobsCalls)

	clock.Add(ttl + time.Second)

	after, err := svc.ListPipelineJobs(context.Background(), "proj", 7, gitlabSettings())
	require.NoError(t, err)
	assert.Len(t, after, 2, "resurrected pipeline must be re-fetched after the short TTL")
	assert.Equal(t, 2, fake.jobsCalls)
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
	assert.Equal(t, 1, fake.jobsCalls, "concurrent misses must collapse to a single fetch")
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
