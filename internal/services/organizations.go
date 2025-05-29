package services

import (
	"context"
	"fmt"

	"github.com/KubeRocketCI/gitfusion/internal/models"
)

type Organizations interface {
	ListUserOrganizations(ctx context.Context, settings GitProviderSettings) ([]models.Organization, error)
}

type OrganizationsService struct {
	organizations    Organizations
	gitServerService *GitServerService
}

func NewOrganizationsService(organizations Organizations, gitServerService *GitServerService) *OrganizationsService {
	return &OrganizationsService{
		organizations:    organizations,
		gitServerService: gitServerService,
	}
}

func (s *OrganizationsService) ListUserOrganizations(
	ctx context.Context,
	gitServerName string,
) ([]models.Organization, error) {
	settings, err := s.gitServerService.GetGitProviderSettings(ctx, gitServerName)
	if err != nil {
		return nil, err
	}

	return s.organizations.ListUserOrganizations(ctx, settings)
}

type MultiProviderOrganizationsService struct {
	providers map[string]Organizations
}

func NewMultiProviderOrganizationsService() *MultiProviderOrganizationsService {
	return &MultiProviderOrganizationsService{
		providers: map[string]Organizations{
			"github":    NewGitHubService(),
			"gitlab":    NewGitlabService(),
			"bitbucket": NewBitbucketService(),
		},
	}
}

func (m *MultiProviderOrganizationsService) ListUserOrganizations(
	ctx context.Context,
	settings GitProviderSettings,
) ([]models.Organization, error) {
	provider, ok := m.providers[settings.GitProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", settings.GitProvider)
	}

	return provider.ListUserOrganizations(ctx, settings)
}
