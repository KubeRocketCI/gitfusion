package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/viccon/sturdyc"
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
	cache     *sturdyc.Client[[]models.Organization]
}

func NewMultiProviderOrganizationsService(
	gitServerService *GitServerService,
) *MultiProviderOrganizationsService {
	service := &MultiProviderOrganizationsService{
		providers: map[string]Organizations{
			"github":    NewGitHubService(),
			"gitlab":    NewGitlabService(),
			"bitbucket": NewBitbucketService(),
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
			go func(setting GitProviderSettings) {
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
	settings GitProviderSettings,
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
