package pipelines

import (
	"context"
	"fmt"

	"github.com/viccon/sturdyc"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/bitbucket"
	"github.com/KubeRocketCI/gitfusion/internal/services/github"
	"github.com/KubeRocketCI/gitfusion/internal/services/gitlab"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

// PipelineProvider defines the interface for CI/CD pipeline operations.
// This interface must be implemented by all git providers (GitLab, GitHub, Bitbucket).
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

// PipelineJobsProvider is an optional capability for providers that expose per-pipeline
// jobs and their logs (GitLab today). Providers that don't implement it return a
// bad-request error via the multi-provider dispatch below.
type PipelineJobsProvider interface {
	ListPipelineJobs(
		ctx context.Context,
		project string,
		pipelineID int,
		settings krci.GitServerSettings,
	) ([]models.PipelineJob, error)

	// GetJobTrace returns the job's trace and whether it was truncated to the size cap.
	GetJobTrace(
		ctx context.Context,
		project string,
		jobID int,
		settings krci.GitServerSettings,
	) (content string, truncated bool, err error)
}

// MultiProviderPipelineService dynamically dispatches pipeline operations
// to the correct provider based on git server settings.
type MultiProviderPipelineService struct {
	providers map[string]PipelineProvider
	cache     *sturdyc.Client[models.PipelinesResponse]
}

// NewMultiProviderPipelineService creates a service with all available provider implementations.
func NewMultiProviderPipelineService() *MultiProviderPipelineService {
	return &MultiProviderPipelineService{
		providers: map[string]PipelineProvider{
			"gitlab":    gitlab.NewGitlabProvider(),
			"github":    github.NewGitHubProvider(),
			"bitbucket": bitbucket.NewBitbucketProvider(),
		},
		cache: cache.NewPipelineCache(),
	}
}

// TriggerPipeline dispatches the pipeline trigger request to the appropriate provider
// based on the git provider specified in settings.
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

// ListPipelines dispatches the pipeline listing request to the appropriate provider
// with caching.
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

// ListPipelineJobs dispatches a pipeline-jobs request to a provider that supports jobs.
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

	return jobsProvider.ListPipelineJobs(ctx, project, pipelineID, settings)
}

// GetJobTrace dispatches a job-trace request to a provider that supports jobs.
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

	return jobsProvider.GetJobTrace(ctx, project, jobID, settings)
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

// GetCache returns the pipeline cache instance for cache management.
func (m *MultiProviderPipelineService) GetCache() *sturdyc.Client[models.PipelinesResponse] {
	return m.cache
}
