package repositories

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
	"github.com/KubeRocketCI/gitfusion/pkg/pointer"
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

// CheckGetRepository reports whether the provider implements getRepository.
func (m *MultiProviderRepositoryService) CheckGetRepository(gitProvider string) error {
	_, err := m.getRepositoryProvider(gitProvider)

	return err
}

// CheckListRepositories reports whether the provider implements listRepositories.
func (m *MultiProviderRepositoryService) CheckListRepositories(gitProvider string) error {
	_, err := m.listRepositoriesProvider(gitProvider)

	return err
}

func (m *MultiProviderRepositoryService) getRepositoryProvider(gitProvider string) (RepositoriesProvider, error) {
	return m.lookup(gitProvider, "getRepository")
}

func (m *MultiProviderRepositoryService) listRepositoriesProvider(gitProvider string) (RepositoriesProvider, error) {
	return m.lookup(gitProvider, "listRepositories")
}

func (m *MultiProviderRepositoryService) lookup(gitProvider, operation string) (RepositoriesProvider, error) {
	provider, ok := m.providers[gitProvider]
	if !ok {
		return nil, gferrors.UnsupportedOperation(gitProvider, operation)
	}

	return provider, nil
}

func (m *MultiProviderRepositoryService) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
) (*models.Repository, error) {
	provider, err := m.getRepositoryProvider(settings.GitProvider)
	if err != nil {
		return nil, err
	}

	return provider.GetRepository(ctx, owner, repo, settings)
}

func (m *MultiProviderRepositoryService) ListRepositories(
	ctx context.Context,
	owner string,
	settings krci.GitServerSettings,
	listOptions models.ListOptions,
) ([]models.Repository, error) {
	provider, err := m.listRepositoriesProvider(settings.GitProvider)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s|%s|%s", settings.GitServerName, owner, pointer.ValueOrEmpty(listOptions.Name))

	fetchFn := func(ctx context.Context) ([]models.Repository, error) {
		return provider.ListRepositories(ctx, owner, settings, listOptions)
	}

	return m.cache.GetOrFetch(ctx, key, fetchFn)
}

// GetCache returns the repository cache instance for cache management.
func (m *MultiProviderRepositoryService) GetCache() *sturdyc.Client[[]models.Repository] {
	return m.cache
}
