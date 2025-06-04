package organizations

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

type OrganizationsService struct {
	organizationsPovider *MultiProviderOrganizationsService
	gitServerService     *krci.GitServerService
}

func NewOrganizationsService(
	organizationsProvider *MultiProviderOrganizationsService,
	gitServerService *krci.GitServerService,
) *OrganizationsService {
	return &OrganizationsService{
		organizationsPovider: organizationsProvider,
		gitServerService:     gitServerService,
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

	return s.organizationsPovider.ListUserOrganizations(ctx, settings)
}
