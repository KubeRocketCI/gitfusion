package pullrequests

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
)

type PullRequestsProvider interface {
	ListPullRequests(
		ctx context.Context,
		owner, repo string,
		settings krci.GitServerSettings,
		opts models.PullRequestListOptions,
	) (*models.PullRequestsResponse, error)
}

type MultiProviderPullRequestsService struct {
	providers map[string]PullRequestsProvider
	cache     *sturdyc.Client[models.PullRequestsResponse]
}

func NewMultiProviderPullRequestsService() *MultiProviderPullRequestsService {
	return &MultiProviderPullRequestsService{
		providers: map[string]PullRequestsProvider{
			"github":    github.NewGitHubProvider(),
			"gitlab":    gitlab.NewGitlabProvider(),
			"bitbucket": bitbucket.NewBitbucketProvider(),
		},
		cache: cache.NewPullRequestCache(),
	}
}

func (m *MultiProviderPullRequestsService) ListPullRequests(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
	opts models.PullRequestListOptions,
) (*models.PullRequestsResponse, error) {
	provider, ok := m.providers[settings.GitProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", settings.GitProvider)
	}

	key := fmt.Sprintf("%s|%s|%s|%s|%d|%d", settings.GitServerName, owner, repo, opts.State, opts.Page, opts.PerPage)

	fetchFn := func(ctx context.Context) (models.PullRequestsResponse, error) {
		resp, err := provider.ListPullRequests(ctx, owner, repo, settings, opts)
		if err != nil {
			return models.PullRequestsResponse{}, err
		}

		return *resp, nil
	}

	result, err := m.cache.GetOrFetch(ctx, key, fetchFn)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetCache returns the pull request cache instance for cache management.
func (m *MultiProviderPullRequestsService) GetCache() *sturdyc.Client[models.PullRequestsResponse] {
	return m.cache
}
