package repositories

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

type RepositoriesService struct {
	gitRepositoriesProvider *MultiProviderRepositoryService
	gitServerService        *krci.GitServerService
}

func NewRepositoriesService(
	gitRepositoriesProvider *MultiProviderRepositoryService,
	gitServerService *krci.GitServerService,
) *RepositoriesService {
	return &RepositoriesService{
		gitRepositoriesProvider: gitRepositoriesProvider,
		gitServerService:        gitServerService,
	}
}

func (r *RepositoriesService) GetRepository(
	ctx context.Context,
	gitServerName, owner, repo string,
) (*models.Repository, error) {
	settings, err := r.gitServerService.GetGitProviderSettings(ctx, gitServerName)
	if err != nil {
		return nil, err
	}

	return r.gitRepositoriesProvider.GetRepository(ctx, owner, repo, settings)
}

func (r *RepositoriesService) ListRepositories(
	ctx context.Context,
	gitServerName, owner string,
	listOptions models.ListOptions,
) ([]models.Repository, error) {
	settings, err := r.gitServerService.GetGitProviderSettings(ctx, gitServerName)
	if err != nil {
		return nil, err
	}

	return r.gitRepositoriesProvider.ListRepositories(ctx, owner, settings, listOptions)
}
