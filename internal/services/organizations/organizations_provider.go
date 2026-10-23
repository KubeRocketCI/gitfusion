package organizations

import (
	"context"

	"github.com/viccon/sturdyc"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/bitbucket"
	"github.com/KubeRocketCI/gitfusion/internal/services/github"
	"github.com/KubeRocketCI/gitfusion/internal/services/gitlab"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

type OrganizationsProvider interface {
	ListUserOrganizations(
		ctx context.Context,
		settings krci.GitServerSettings,
	) ([]models.Organization, error)
}

type MultiProviderOrganizationsService struct {
	providers map[string]OrganizationsProvider
	cache     *sturdyc.Client[[]models.Organization]
}

func NewMultiProviderOrganizationsService() *MultiProviderOrganizationsService {
	return &MultiProviderOrganizationsService{
		providers: map[string]OrganizationsProvider{
			"github":    github.NewGitHubProvider(),
			"gitlab":    gitlab.NewGitlabProvider(),
			"bitbucket": bitbucket.NewBitbucketProvider(),
		},
		cache: cache.NewOrganizationCache(),
	}
}

// CheckListUserOrganizations reports whether the provider implements listUserOrganizations.
func (m *MultiProviderOrganizationsService) CheckListUserOrganizations(gitProvider string) error {
	_, err := m.listUserOrganizationsProvider(gitProvider)

	return err
}

func (m *MultiProviderOrganizationsService) listUserOrganizationsProvider(
	gitProvider string,
) (OrganizationsProvider, error) {
	provider, ok := m.providers[gitProvider]
	if !ok {
		return nil, gferrors.UnsupportedOperation(gitProvider, "listUserOrganizations")
	}

	return provider, nil
}

func (m *MultiProviderOrganizationsService) ListUserOrganizations(
	ctx context.Context,
	settings krci.GitServerSettings,
) ([]models.Organization, error) {
	provider, err := m.listUserOrganizationsProvider(settings.GitProvider)
	if err != nil {
		return nil, err
	}

	fetchFn := func(ctx context.Context) ([]models.Organization, error) {
		return provider.ListUserOrganizations(ctx, settings)
	}

	return m.cache.GetOrFetch(ctx, settings.GitServerName, fetchFn)
}

// GetCache returns the organization cache instance for cache management.
func (m *MultiProviderOrganizationsService) GetCache() *sturdyc.Client[[]models.Organization] {
	return m.cache
}
