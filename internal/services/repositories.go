package services

import (
	"context"
	"fmt"

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
		owner string,
		settings GitProviderSettings,
		listOptions ListOptions,
	) ([]models.Repository, error)
}

type GitProviderSettings struct {
	Url         string
	Token       string
	GitProvider string
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

func (r *RepositoriesService) ListRepositories(
	ctx context.Context,
	gitServerName, owner string,
	listOptions ListOptions,
) ([]models.Repository, error) {
	settings, err := r.gitServerService.GetGitProviderSettings(ctx, gitServerName)
	if err != nil {
		return nil, err
	}

	return r.gitRepositoriesService.ListRepositories(ctx, owner, settings, listOptions)
}

// MultiProviderRepositoryService dynamically dispatches to the correct provider implementation.
type MultiProviderRepositoryService struct {
	providers map[string]Repositories
}

func NewMultiProviderRepositoryService() *MultiProviderRepositoryService {
	return &MultiProviderRepositoryService{
		providers: map[string]Repositories{
			"github":    NewGitHubService(),
			"gitlab":    NewGitlabService(),
			"bitbucket": NewBitbucketService(),
		},
	}
}

func (m *MultiProviderRepositoryService) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings GitProviderSettings,
) (*models.Repository, error) {
	provider, ok := m.providers[settings.GitProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", settings.GitProvider)
	}

	return provider.GetRepository(ctx, owner, repo, settings)
}

func (m *MultiProviderRepositoryService) ListRepositories(
	ctx context.Context,
	owner string,
	settings GitProviderSettings,
	listOptions ListOptions,
) ([]models.Repository, error) {
	provider, ok := m.providers[settings.GitProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", settings.GitProvider)
	}

	return provider.ListRepositories(ctx, owner, settings, listOptions)
}
