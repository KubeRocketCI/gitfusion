package cache

import (
	"fmt"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/viccon/sturdyc"
)

// Manager provides centralized cache management for all cache instances.
type Manager struct {
	repositoryCache   *sturdyc.Client[[]models.Repository]
	organizationCache *sturdyc.Client[[]models.Organization]
	branchCache       *sturdyc.Client[[]models.Branch]
	pullRequestCache  *sturdyc.Client[models.PullRequestsResponse]
}

// NewManager creates a new cache manager with all cache instances.
func NewManager(
	repositoryCache *sturdyc.Client[[]models.Repository],
	organizationCache *sturdyc.Client[[]models.Organization],
	branchCache *sturdyc.Client[[]models.Branch],
	pullRequestCache *sturdyc.Client[models.PullRequestsResponse],
) *Manager {
	return &Manager{
		repositoryCache:   repositoryCache,
		organizationCache: organizationCache,
		branchCache:       branchCache,
		pullRequestCache:  pullRequestCache,
	}
}

// InvalidateCache invalidates the specified cache type.
func (m *Manager) InvalidateCache(endpoint string) error {
	switch endpoint {
	case "repositories":
		keys := m.repositoryCache.ScanKeys()
		for _, key := range keys {
			m.repositoryCache.Delete(key)
		}

		return nil
	case "organizations":
		keys := m.organizationCache.ScanKeys()
		for _, key := range keys {
			m.organizationCache.Delete(key)
		}

		return nil
	case "branches":
		keys := m.branchCache.ScanKeys()
		for _, key := range keys {
			m.branchCache.Delete(key)
		}

		return nil
	case "pullrequests":
		keys := m.pullRequestCache.ScanKeys()
		for _, key := range keys {
			m.pullRequestCache.Delete(key)
		}

		return nil
	default:
		return fmt.Errorf("unsupported endpoint: %s", endpoint)
	}
}

// GetSupportedEndpoints returns a list of supported cache endpoints.
func (m *Manager) GetSupportedEndpoints() []string {
	return []string{"repositories", "organizations", "branches", "pullrequests"}
}
