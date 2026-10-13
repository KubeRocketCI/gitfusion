package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viccon/sturdyc"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
)

// managerCaches holds the caches handed to NewManager so tests can seed and inspect them.
type managerCaches struct {
	repositories  *sturdyc.Client[[]models.Repository]
	organizations *sturdyc.Client[[]models.Organization]
	branches      *sturdyc.Client[[]models.Branch]
	pullRequests  *sturdyc.Client[models.PullRequestsResponse]
	pipelines     *sturdyc.Client[models.PipelinesResponse]
	pipelineJobs  *sturdyc.Client[[]models.PipelineJob]
	jobTraces     *TerminalAwareCache[JobTrace]
}

func newManagerCaches() managerCaches {
	return managerCaches{
		repositories:  NewRepositoryCache(),
		organizations: NewOrganizationCache(),
		branches:      NewBranchCache(),
		pullRequests:  NewPullRequestCache(),
		pipelines:     NewPipelineCache(),
		pipelineJobs:  NewPipelineJobsCache(),
		jobTraces:     NewPipelineJobTraceCache(),
	}
}

func (c managerCaches) manager() *Manager {
	return NewManager(
		c.repositories, c.organizations, c.branches, c.pullRequests, c.pipelines, c.pipelineJobs, c.jobTraces,
	)
}

// cacheEntry is one seeded entry: how to store it and whether it is still there.
type cacheEntry struct {
	name string
	set  func()
	held func() bool
}

func found[T any](_ T, ok bool) bool { return ok }

// entries lists one entry per cache, and one per tier of the job trace cache.
func (c managerCaches) entries() []cacheEntry {
	return []cacheEntry{
		{
			name: "repositories",
			set:  func() { c.repositories.Set("k", []models.Repository{{Name: "api"}}) },
			held: func() bool { return found(c.repositories.Get("k")) },
		},
		{
			name: "organizations",
			set:  func() { c.organizations.Set("k", []models.Organization{{Name: "acme"}}) },
			held: func() bool { return found(c.organizations.Get("k")) },
		},
		{
			name: "branches",
			set:  func() { c.branches.Set("k", []models.Branch{{Name: "main"}}) },
			held: func() bool { return found(c.branches.Get("k")) },
		},
		{
			name: "pullrequests",
			set:  func() { c.pullRequests.Set("k", models.PullRequestsResponse{}) },
			held: func() bool { return found(c.pullRequests.Get("k")) },
		},
		{
			name: "pipelines",
			set:  func() { c.pipelines.Set("k", models.PipelinesResponse{}) },
			held: func() bool { return found(c.pipelines.Get("k")) },
		},
		{
			name: "pipeline jobs",
			set:  func() { c.pipelineJobs.Set("k", []models.PipelineJob{{Id: "1"}}) },
			held: func() bool { return found(c.pipelineJobs.Get("k")) },
		},
		{
			name: "running trace",
			set:  func() { c.jobTraces.Set("running", JobTrace{Content: "log"}, false) },
			held: func() bool { return found(c.jobTraces.Get("running")) },
		},
		{
			name: "finished trace",
			set:  func() { c.jobTraces.Set("finished", JobTrace{Content: "log"}, true) },
			held: func() bool { return found(c.jobTraces.Get("finished")) },
		},
	}
}

func (c managerCaches) seed() {
	for _, e := range c.entries() {
		e.set()
	}
}

// emptied lists, in entries order, the caches that no longer hold their seeded entry.
func (c managerCaches) emptied() []string {
	var emptied []string

	for _, e := range c.entries() {
		if !e.held() {
			emptied = append(emptied, e.name)
		}
	}

	return emptied
}

func TestManager_InvalidateCacheClearsOnlyTheEndpointCaches(t *testing.T) {
	tests := []struct {
		endpoint models.InvalidateCacheParamsEndpoint
		cleared  []string
	}{
		{endpoint: models.Repositories, cleared: []string{"repositories"}},
		{endpoint: models.Organizations, cleared: []string{"organizations"}},
		{endpoint: models.Branches, cleared: []string{"branches"}},
		{endpoint: models.Pullrequests, cleared: []string{"pullrequests"}},
		{
			endpoint: models.Pipelines,
			cleared:  []string{"pipelines", "pipeline jobs", "running trace", "finished trace"},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.endpoint), func(t *testing.T) {
			caches := newManagerCaches()
			caches.seed()
			require.Empty(t, caches.emptied())

			require.NoError(t, caches.manager().InvalidateCache(string(tt.endpoint)))

			assert.Equal(t, tt.cleared, caches.emptied())
		})
	}
}

func TestManager_InvalidateCacheRejectsUnknownEndpoint(t *testing.T) {
	caches := newManagerCaches()
	caches.seed()
	require.Empty(t, caches.emptied())

	err := caches.manager().InvalidateCache("foo")

	require.ErrorIs(t, err, gferrors.ErrBadRequest)
	assert.Contains(t, err.Error(), "unsupported endpoint foo")
	assert.Empty(t, caches.emptied())
}
