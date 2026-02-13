package branches

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
	"github.com/KubeRocketCI/gitfusion/pkg/pointer"
)

type BranchesProvider interface {
	ListBranches(
		ctx context.Context,
		owner, repo string,
		settings krci.GitServerSettings,
		opts models.ListOptions,
	) ([]models.Branch, error)
}

type MultiProviderBranchesService struct {
	providers map[string]BranchesProvider
	cache     *sturdyc.Client[[]models.Branch]
}

func NewMultiProviderBranchesService() *MultiProviderBranchesService {
	return &MultiProviderBranchesService{
		providers: map[string]BranchesProvider{
			"github":    github.NewGitHubProvider(),
			"gitlab":    gitlab.NewGitlabProvider(),
			"bitbucket": bitbucket.NewBitbucketProvider(),
		},
		cache: cache.NewBranchCache(),
	}
}

func (m *MultiProviderBranchesService) ListBranches(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
	opts models.ListOptions,
) ([]models.Branch, error) {
	provider, ok := m.providers[settings.GitProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", settings.GitProvider)
	}

	key := fmt.Sprintf("%s|%s|%s|%s", settings.GitServerName, owner, repo, pointer.ValueOrEmpty(opts.Name))

	fetchFn := func(ctx context.Context) ([]models.Branch, error) {
		return provider.ListBranches(ctx, owner, repo, settings, opts)
	}

	return m.cache.GetOrFetch(ctx, key, fetchFn)
}

// GetCache returns the branch cache instance for cache management.
func (m *MultiProviderBranchesService) GetCache() *sturdyc.Client[[]models.Branch] {
	return m.cache
}
