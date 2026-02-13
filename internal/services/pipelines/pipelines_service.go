// Package pipelines provides services for triggering CI/CD pipelines across different git providers.
package pipelines

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

// PipelinesService handles pipeline operations by coordinating between
// git server settings and provider-specific implementations.
type PipelinesService struct {
	gitServerService  *krci.GitServerService
	pipelinesProvider *MultiProviderPipelineService
}

// NewPipelinesService creates a new PipelinesService.
func NewPipelinesService(
	pipelinesProvider *MultiProviderPipelineService,
	gitServerService *krci.GitServerService,
) *PipelinesService {
	return &PipelinesService{
		gitServerService:  gitServerService,
		pipelinesProvider: pipelinesProvider,
	}
}

// TriggerPipeline triggers a CI/CD pipeline for the specified git server, project, and ref.
// It fetches git server settings from Kubernetes and delegates to the appropriate provider.
func (s *PipelinesService) TriggerPipeline(
	ctx context.Context,
	gitServerName string,
	project string,
	ref string,
	variables []models.PipelineVariable,
) (*models.PipelineResponse, error) {
	// Get settings from K8s (GitServer CR + Secret)
	settings, err := s.gitServerService.GetGitProviderSettings(ctx, gitServerName)
	if err != nil {
		return nil, err
	}

	// Delegate to multi-provider service
	return s.pipelinesProvider.TriggerPipeline(ctx, project, ref, variables, settings)
}

// ListPipelines lists CI/CD pipelines for the specified git server and project.
// It fetches git server settings from Kubernetes and delegates to the appropriate provider.
func (s *PipelinesService) ListPipelines(
	ctx context.Context,
	gitServerName string,
	project string,
	opts models.PipelineListOptions,
) (*models.PipelinesResponse, error) {
	settings, err := s.gitServerService.GetGitProviderSettings(ctx, gitServerName)
	if err != nil {
		return nil, err
	}

	return s.pipelinesProvider.ListPipelines(ctx, project, settings, opts)
}

// GetProvider returns the underlying multi-provider service for direct access to its cache.
func (s *PipelinesService) GetProvider() *MultiProviderPipelineService {
	return s.pipelinesProvider
}
