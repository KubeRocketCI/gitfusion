package services

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/models"
)

type Repositories interface {
	GetRepository(ctx context.Context, token, repositoryID string) (models.Repository, error)
}

type RepositoriesService struct {
	gitRepositoriesService Repositories
	gitServerService       *GitServerService
}

func NewRepositoriesService(gitRepositoriesService Repositories, gitServerService *GitServerService) *RepositoriesService {
	return &RepositoriesService{
		gitRepositoriesService: gitRepositoriesService,
		gitServerService:       gitServerService,
	}
}

func (r *RepositoriesService) GetRepository(ctx context.Context, gitServerName, repositoryID string) (models.Repository, error) {
	return models.Repository{}, nil

	token, err := r.gitServerService.GetGitProviderToken(ctx, gitServerName)
	if err != nil {
		return models.Repository{}, err
	}

	return r.gitRepositoriesService.GetRepository(ctx, token, repositoryID)
}
