package pipelines

import (
	"context"
	"fmt"

	"github.com/viccon/sturdyc"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
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
		return nil, fmt.Errorf("unsupported provider: %s", settings.GitProvider)
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
		return nil, fmt.Errorf("unsupported provider: %s", settings.GitProvider)
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

// GetCache returns the pipeline cache instance for cache management.
func (m *MultiProviderPipelineService) GetCache() *sturdyc.Client[models.PipelinesResponse] {
	return m.cache
}
