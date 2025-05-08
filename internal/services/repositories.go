package services

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/models"
)

type Repositories interface {
	GetRepository(
		ctx context.Context,
		token, owner, repo string,
	) (*models.Repository, error)
	ListOrganizationsRepositories(
		ctx context.Context,
		token, org string,
		listOptions ListOptions,
	) ([]models.Repository, error)
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
	token, err := r.gitServerService.GetGitProviderToken(ctx, gitServerName)
	if err != nil {
		return nil, err
	}

	return r.gitRepositoriesService.GetRepository(ctx, token, owner, repo)
}

func (r *RepositoriesService) ListOrganizationsRepositories(
	ctx context.Context,
	gitServerName, org string,
	listOptions ListOptions,
) ([]models.Repository, error) {
	token, err := r.gitServerService.GetGitProviderToken(ctx, gitServerName)
	if err != nil {
		return nil, err
	}

	return r.gitRepositoriesService.ListOrganizationsRepositories(ctx, token, org, listOptions)
}
