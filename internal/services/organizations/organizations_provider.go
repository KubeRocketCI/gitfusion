package organizations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/bitbucket"
	"github.com/KubeRocketCI/gitfusion/internal/services/github"
	"github.com/KubeRocketCI/gitfusion/internal/services/gitlab"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	"github.com/viccon/sturdyc"
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

func NewMultiProviderOrganizationsService(
	gitServerService *krci.GitServerService,
) *MultiProviderOrganizationsService {
	service := &MultiProviderOrganizationsService{
		providers: map[string]OrganizationsProvider{
			"github":    github.NewGitHubProvider(),
			"gitlab":    gitlab.NewGitlabProvider(),
			"bitbucket": bitbucket.NewBitbucketProvider(),
		},
		cache: cache.NewOrganizationCache(),
	}

	ctx := context.Background()

	// Warm up the organization cache for all configured providers in the background.
	// This helps to ensure that the cache is populated before any requests are made.
	go func() {
		settings, err := gitServerService.GetGitProviderSettingsList(ctx)
		if err != nil {
			slog.Error("Failed to get git provider settings", "error", err)
			return
		}

		for _, setting := range settings {
			// It's safe to spawn new goroutines for each provider because we may have only a few providers.
			go func(setting krci.GitServerSettings) {
				if _, fetchErr := service.ListUserOrganizations(ctx, setting); fetchErr != nil {
					slog.Error("Failed to list user organizations", "provider", setting.GitProvider, "error", fetchErr)

					return
				}

				slog.Info("Warmed up organization cache", "provider", setting.GitProvider, "gitServerName", setting.GitServerName)
			}(setting)
		}
	}()

	return service
}

func (m *MultiProviderOrganizationsService) ListUserOrganizations(
	ctx context.Context,
	settings krci.GitServerSettings,
) ([]models.Organization, error) {
	provider, ok := m.providers[settings.GitProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", settings.GitProvider)
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
