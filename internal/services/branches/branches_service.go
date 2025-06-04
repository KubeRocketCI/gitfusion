package branches

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

type BranchesService struct {
	branchesProvider *MultiProviderBranchesService
	gitServerService *krci.GitServerService
}

func NewBranchesService(
	branchesProvider *MultiProviderBranchesService,
	gitServerService *krci.GitServerService,
) *BranchesService {
	return &BranchesService{
		branchesProvider: branchesProvider,
		gitServerService: gitServerService,
	}
}

func (s *BranchesService) ListBranches(
	ctx context.Context,
	gitServerName, owner, repoName string,
	opts models.ListOptions,
) ([]models.Branch, error) {
	settings, err := s.gitServerService.GetGitProviderSettings(ctx, gitServerName)
	if err != nil {
		return nil, err
	}

	return s.branchesProvider.ListBranches(ctx, owner, repoName, settings, opts)
}
