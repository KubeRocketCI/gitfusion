package services

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/models"
)

type Repositories interface {
	GetRepository(
		ctx context.Context,
		owner, repo string,
		settings GitProviderSettings,
	) (*models.Repository, error)
	ListRepositories(
		ctx context.Context,
		org string,
		settings GitProviderSettings,
		listOptions ListOptions,
	) ([]models.Repository, error)
}

type GitProviderSettings struct {
	Url   string
	Token string
}

type RepositoriesService struct {
	gitRepositoriesService Repositories
	gitServerService       *GitServerService
}

func NewRepositoriesService(
	gitRepositoriesService Repositories,
	gitServerService *GitServerService,
) *RepositoriesService {
	return &RepositoriesService{
		gitRepositoriesService: gitRepositoriesService,
		gitServerService:       gitServerService,
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

	return r.gitRepositoriesService.GetRepository(ctx, owner, repo, settings)
}

func (r *RepositoriesService) ListOrganizationsRepositories(
	ctx context.Context,
	gitServerName, org string,
	listOptions ListOptions,
) ([]models.Repository, error) {
	settings, err := r.gitServerService.GetGitProviderSettings(ctx, gitServerName)
	if err != nil {
		return nil, err
	}

	return r.gitRepositoriesService.ListRepositories(ctx, org, settings, listOptions)
}
