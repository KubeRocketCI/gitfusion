package pipelines

import (
	"context"
	"fmt"
	"strconv"

	"github.com/viccon/sturdyc"
	"golang.org/x/sync/singleflight"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/bitbucket"
	"github.com/KubeRocketCI/gitfusion/internal/services/github"
	"github.com/KubeRocketCI/gitfusion/internal/services/gitlab"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

type PipelineProvider interface {
	TriggerPipeline(
		ctx context.Context,
		project string,
		ref string,
		variables []models.PipelineVariable,
		settings krci.GitServerSettings,
	) (*models.PipelineResponse, error)

	ListPipelines(
		ctx context.Context,
		project string,
		settings krci.GitServerSettings,
		opts models.PipelineListOptions,
	) (*models.PipelinesResponse, error)
}

// PipelineJobsProvider is an optional capability; providers that don't implement it make the
// dispatch below return a bad-request error.
type PipelineJobsProvider interface {
	ListPipelineJobs(
		ctx context.Context,
		project string,
		pipelineID int,
		settings krci.GitServerSettings,
	) ([]models.PipelineJob, error)

	GetJobTrace(
		ctx context.Context,
		project string,
		jobID int,
		settings krci.GitServerSettings,
	) (content string, truncated bool, err error)
}

type MultiProviderPipelineService struct {
	providers  map[string]PipelineProvider
	cache      *sturdyc.Client[models.PipelinesResponse]
	jobsCache  *sturdyc.Client[[]models.PipelineJob]
	traceCache *cache.TerminalAwareCache[cache.JobTrace]

	// terminalJobs lets GetJobTrace (which receives only a job ID) tell whether a trace is final.
	terminalJobs *sturdyc.Client[bool]

	// traceGroup de-duplicates concurrent trace fetches; sturdyc does not de-duplicate the trace
	// cache's Get/Set path (the jobs cache gets de-duplication from GetOrFetch).
	traceGroup singleflight.Group
}

func NewMultiProviderPipelineService() *MultiProviderPipelineService {
	return &MultiProviderPipelineService{
		providers: map[string]PipelineProvider{
			"gitlab":    gitlab.NewGitlabProvider(),
			"github":    github.NewGitHubProvider(),
			"bitbucket": bitbucket.NewBitbucketProvider(),
		},
		cache:        cache.NewPipelineCache(),
		jobsCache:    cache.NewPipelineJobsCache(),
		traceCache:   cache.NewPipelineJobTraceCache(),
		terminalJobs: cache.NewTerminalJobsCache(),
	}
}

// terminalJobStatuses are GitLab job statuses that never change again (a retry yields a new job ID).
var terminalJobStatuses = map[string]bool{
	"success":  true,
	"failed":   true,
	"canceled": true,
	"skipped":  true,
}

func isTerminalJobStatus(status string) bool {
	return terminalJobStatuses[status]
}

// terminalJobKey namespaces the marker by git server so job IDs from different instances can't collide.
func terminalJobKey(gitServerName string, jobID string) string {
	return fmt.Sprintf("%s|%s", gitServerName, jobID)
}

func (m *MultiProviderPipelineService) TriggerPipeline(
	ctx context.Context,
	project string,
	ref string,
	variables []models.PipelineVariable,
	settings krci.GitServerSettings,
) (*models.PipelineResponse, error) {
	provider, ok := m.providers[settings.GitProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %s: %w", settings.GitProvider, gferrors.ErrBadRequest)
	}

	return provider.TriggerPipeline(ctx, project, ref, variables, settings)
}

func (m *MultiProviderPipelineService) ListPipelines(
	ctx context.Context,
	project string,
	settings krci.GitServerSettings,
	opts models.PipelineListOptions,
) (*models.PipelinesResponse, error) {
	provider, ok := m.providers[settings.GitProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %s: %w", settings.GitProvider, gferrors.ErrBadRequest)
	}

	refKey := ""
	if opts.Ref != nil {
		refKey = *opts.Ref
	}

	statusKey := ""
	if opts.Status != nil {
		statusKey = *opts.Status
	}

	key := fmt.Sprintf("%s|%s|%s|%s|%d|%d", settings.GitServerName, project, refKey, statusKey, opts.Page, opts.PerPage)

	fetchFn := func(ctx context.Context) (models.PipelinesResponse, error) {
		resp, err := provider.ListPipelines(ctx, project, settings, opts)
		if err != nil {
			return models.PipelinesResponse{}, err
		}

		return *resp, nil
	}

	result, err := m.cache.GetOrFetch(ctx, key, fetchFn)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// ListPipelineJobs serves a pipeline's jobs from a short-TTL cache (GetOrFetch de-duplicates
// concurrent misses) and records finished jobs so GetJobTrace can long-cache their traces.
func (m *MultiProviderPipelineService) ListPipelineJobs(
	ctx context.Context,
	project string,
	pipelineID int,
	settings krci.GitServerSettings,
) ([]models.PipelineJob, error) {
	jobsProvider, err := m.jobsProvider(settings.GitProvider)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s|%s|%d", settings.GitServerName, project, pipelineID)

	return m.jobsCache.GetOrFetch(ctx, key, func(ctx context.Context) ([]models.PipelineJob, error) {
		jobs, err := jobsProvider.ListPipelineJobs(ctx, project, pipelineID, settings)
		if err != nil {
			return nil, err
		}

		m.markTerminalJobs(settings.GitServerName, jobs)

		return jobs, nil
	})
}

// markTerminalJobs records every finished job so GetJobTrace can long-cache its trace.
func (m *MultiProviderPipelineService) markTerminalJobs(gitServerName string, jobs []models.PipelineJob) {
	for i := range jobs {
		if isTerminalJobStatus(jobs[i].Status) {
			m.terminalJobs.Set(terminalJobKey(gitServerName, jobs[i].Id), true)
		}
	}
}

// GetJobTrace serves a job's trace; only complete traces within cache.MaxCacheableTraceBytes are
// cached (truncated or oversized traces bypass the cache).
func (m *MultiProviderPipelineService) GetJobTrace(
	ctx context.Context,
	project string,
	jobID int,
	settings krci.GitServerSettings,
) (string, bool, error) {
	jobsProvider, err := m.jobsProvider(settings.GitProvider)
	if err != nil {
		return "", false, err
	}

	key := fmt.Sprintf("%s|%s|%d", settings.GitServerName, project, jobID)

	if cached, ok := m.traceCache.Get(key); ok {
		return cached.Content, cached.Truncated, nil
	}

	// singleflight collapses concurrent misses for the same job into a single fetch+store.
	v, err, _ := m.traceGroup.Do(key, func() (any, error) {
		// Re-check under the flight: a just-finished flight may have populated the cache.
		if cached, ok := m.traceCache.Get(key); ok {
			return cached, nil
		}

		content, truncated, ferr := jobsProvider.GetJobTrace(ctx, project, jobID, settings)
		if ferr != nil {
			return cache.JobTrace{}, ferr
		}

		trace := cache.JobTrace{Content: content, Truncated: truncated}

		if !truncated && len(content) <= cache.MaxCacheableTraceBytes {
			_, terminal := m.terminalJobs.Get(terminalJobKey(settings.GitServerName, strconv.Itoa(jobID)))
			m.traceCache.Set(key, trace, terminal)
		}

		return trace, nil
	})
	if err != nil {
		return "", false, err
	}

	trace, _ := v.(cache.JobTrace)

	return trace.Content, trace.Truncated, nil
}

// jobsProvider resolves a provider that supports per-pipeline jobs/logs, or a
// bad-request error if the configured provider doesn't (e.g. GitHub/Bitbucket).
func (m *MultiProviderPipelineService) jobsProvider(gitProvider string) (PipelineJobsProvider, error) {
	provider, ok := m.providers[gitProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %s: %w", gitProvider, gferrors.ErrBadRequest)
	}

	jobsProvider, ok := provider.(PipelineJobsProvider)
	if !ok {
		return nil, fmt.Errorf("provider %s does not support pipeline jobs: %w", gitProvider, gferrors.ErrBadRequest)
	}

	return jobsProvider, nil
}

func (m *MultiProviderPipelineService) GetCache() *sturdyc.Client[models.PipelinesResponse] {
	return m.cache
}

func (m *MultiProviderPipelineService) GetJobsCache() *sturdyc.Client[[]models.PipelineJob] {
	return m.jobsCache
}

func (m *MultiProviderPipelineService) GetTraceCache() *cache.TerminalAwareCache[cache.JobTrace] {
	return m.traceCache
}
