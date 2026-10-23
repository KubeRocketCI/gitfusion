package organizations

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
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
	settings, err := s.gitServerService.Resolve(ctx, gitServerName, s.organizationsPovider.CheckListUserOrganizations)
	if err != nil {
		return nil, err
	}

	return s.organizationsPovider.ListUserOrganizations(ctx, settings)
}

// WarmUp fills the organization cache for every GitServer whose provider supports listUserOrganizations and
// returns when all fetches have finished. A failing GitServer is logged and does not stop the others.
func (s *OrganizationsService) WarmUp(ctx context.Context) {
	names, err := s.gitServerService.GitServerNames(ctx)
	if err != nil {
		slog.Error("Failed to list git servers", "error", err)

		return
	}

	var wg sync.WaitGroup

	for _, name := range names {
		wg.Go(func() { s.warmUpGitServer(ctx, name) })
	}

	wg.Wait()
}

func (s *OrganizationsService) warmUpGitServer(ctx context.Context, name string) {
	settings, err := s.gitServerService.Resolve(ctx, name, s.organizationsPovider.CheckListUserOrganizations)

	switch {
	case errors.Is(err, gferrors.ErrUnsupportedOperation):
		slog.Debug("Skipped organization cache warm-up", "gitServerName", name, "reason", err)

		return
	case err != nil:
		slog.Error("Failed to resolve git server for organization cache warm-up", "gitServerName", name, "error", err)

		return
	}

	if _, err = s.organizationsPovider.ListUserOrganizations(ctx, settings); err != nil {
		slog.Error("Failed to list user organizations", "gitServerName", name, "error", err)

		return
	}

	slog.Info("Warmed up organization cache", "provider", settings.GitProvider, "gitServerName", name)
}

// GetProvider returns the underlying multi-provider service for direct access to its cache.
func (s *OrganizationsService) GetProvider() *MultiProviderOrganizationsService {
	return s.organizationsPovider
}
