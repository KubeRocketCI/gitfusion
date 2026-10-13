package cache

import (
	"fmt"

	"github.com/viccon/sturdyc"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
)

// Manager provides centralized cache management for all cache instances.
type Manager struct {
	invalidators map[models.InvalidateCacheParamsEndpoint]func()
}

// NewManager creates a new cache manager with all cache instances.
func NewManager(
	repositoryCache *sturdyc.Client[[]models.Repository],
	organizationCache *sturdyc.Client[[]models.Organization],
	branchCache *sturdyc.Client[[]models.Branch],
	pullRequestCache *sturdyc.Client[models.PullRequestsResponse],
	pipelineCache *sturdyc.Client[models.PipelinesResponse],
	pipelineJobsCache *sturdyc.Client[[]models.PipelineJob],
	pipelineJobTrace *TerminalAwareCache[JobTrace],
) *Manager {
	return &Manager{
		invalidators: map[models.InvalidateCacheParamsEndpoint]func(){
			models.Repositories:  func() { purge(repositoryCache) },
			models.Organizations: func() { purge(organizationCache) },
			models.Branches:      func() { purge(branchCache) },
			models.Pullrequests:  func() { purge(pullRequestCache) },
			models.Pipelines: func() {
				purge(pipelineCache)
				purge(pipelineJobsCache)
				pipelineJobTrace.Invalidate()
			},
		},
	}
}

// InvalidateCache returns an error wrapping gferrors.ErrBadRequest when endpoint is unknown.
// Clearing is best-effort: entries written during the scan can survive.
func (m *Manager) InvalidateCache(endpoint string) error {
	invalidate, ok := m.invalidators[models.InvalidateCacheParamsEndpoint(endpoint)]
	if !ok {
		return fmt.Errorf("unsupported endpoint %s: %w", endpoint, gferrors.ErrBadRequest)
	}

	invalidate()

	return nil
}

func purge[T any](c *sturdyc.Client[T]) {
	for _, key := range c.ScanKeys() {
		c.Delete(key)
	}
}
