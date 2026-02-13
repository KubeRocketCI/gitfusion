package pullrequests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/KubeRocketCI/gitfusion/internal/models"
)

func defaultOpts() models.PullRequestListOptions {
	return models.PullRequestListOptions{
		State:   "open",
		Page:    1,
		PerPage: 20,
	}
}

func TestNewPullRequestsService(t *testing.T) {
	t.Run("creates service with valid dependencies", func(t *testing.T) {
		multiProvider := NewMultiProviderPullRequestsService()
		assert.NotNil(t, multiProvider)

		service := NewPullRequestsService(multiProvider, nil)
		assert.NotNil(t, service)
		assert.NotNil(t, service.pullRequestsProvider)
	})
}

func TestPullRequestsService_GetProvider(t *testing.T) {
	multiProvider := NewMultiProviderPullRequestsService()
	service := NewPullRequestsService(multiProvider, nil)

	provider := service.GetProvider()
	assert.NotNil(t, provider)
	assert.Equal(t, multiProvider, provider)
}
