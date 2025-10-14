package pipelines

import (
	"context"
	"fmt"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/gitlab"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

// PipelineProvider defines the interface for triggering CI/CD pipelines.
// This interface must be implemented by all git providers (GitLab, GitHub, Bitbucket).
type PipelineProvider interface {
	TriggerPipeline(
		ctx context.Context,
		project string,
		ref string,
		variables []models.PipelineVariable,
		settings krci.GitServerSettings,
	) (*models.PipelineResponse, error)
}

// MultiProviderPipelineService dynamically dispatches pipeline operations
// to the correct provider based on git server settings.
type MultiProviderPipelineService struct {
	providers map[string]PipelineProvider
}

// NewMultiProviderPipelineService creates a service with all available provider implementations.
func NewMultiProviderPipelineService() *MultiProviderPipelineService {
	return &MultiProviderPipelineService{
		providers: map[string]PipelineProvider{
			"gitlab": gitlab.NewGitlabProvider(),
			// Ready to add more providers:
			// "github": github.NewGitHubProvider(),
			// "bitbucket": bitbucket.NewBitbucketProvider(),
		},
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
