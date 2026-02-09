package pullrequests

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

type PullRequestsService struct {
	pullRequestsProvider *MultiProviderPullRequestsService
	gitServerService     *krci.GitServerService
}

func NewPullRequestsService(
	pullRequestsProvider *MultiProviderPullRequestsService,
	gitServerService *krci.GitServerService,
) *PullRequestsService {
	return &PullRequestsService{
		pullRequestsProvider: pullRequestsProvider,
		gitServerService:     gitServerService,
	}
}

func (s *PullRequestsService) ListPullRequests(
	ctx context.Context,
	gitServerName, owner, repoName string,
	opts models.PullRequestListOptions,
) (*models.PullRequestsResponse, error) {
	settings, err := s.gitServerService.GetGitProviderSettings(ctx, gitServerName)
	if err != nil {
		return nil, err
	}

	return s.pullRequestsProvider.ListPullRequests(ctx, owner, repoName, settings, opts)
}

// GetProvider returns the underlying multi-provider service for direct access to its cache.
func (s *PullRequestsService) GetProvider() *MultiProviderPullRequestsService {
	return s.pullRequestsProvider
}
