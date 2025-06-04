package repositories

import (
	"context"
	"fmt"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/bitbucket"
	"github.com/KubeRocketCI/gitfusion/internal/services/github"
	"github.com/KubeRocketCI/gitfusion/internal/services/gitlab"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	"github.com/KubeRocketCI/gitfusion/pkg/pointer"
	"github.com/viccon/sturdyc"
)

type RepositoriesProvider interface {
	GetRepository(
		ctx context.Context,
		owner, repo string,
		settings krci.GitServerSettings,
	) (*models.Repository, error)
	ListRepositories(
		ctx context.Context,
		owner string,
		settings krci.GitServerSettings,
		listOptions models.ListOptions,
	) ([]models.Repository, error)
}

// MultiProviderRepositoryService dynamically dispatches to the correct provider implementation.
type MultiProviderRepositoryService struct {
	providers map[string]RepositoriesProvider
	cache     *sturdyc.Client[[]models.Repository]
}

func NewMultiProviderRepositoryService() *MultiProviderRepositoryService {
	return &MultiProviderRepositoryService{
		providers: map[string]RepositoriesProvider{
			"github":    github.NewGitHubProvider(),
			"gitlab":    gitlab.NewGitlabProvider(),
			"bitbucket": bitbucket.NewBitbucketProvider(),
		},
		cache: cache.NewRepositoryCache(),
	}
}

func (m *MultiProviderRepositoryService) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
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
	settings krci.GitServerSettings,
	listOptions models.ListOptions,
) ([]models.Repository, error) {
	provider, ok := m.providers[settings.GitProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", settings.GitProvider)
	}

	key := fmt.Sprintf("%s|%s|%s", settings.GitServerName, owner, pointer.ValueOrEmpty(listOptions.Name))

	fetchFn := func(ctx context.Context) ([]models.Repository, error) {
		return provider.ListRepositories(ctx, owner, settings, listOptions)
	}

	return m.cache.GetOrFetch(ctx, key, fetchFn)
}
