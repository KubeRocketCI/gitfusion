package pullrequests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

func TestNewMultiProviderPullRequestsService(t *testing.T) {
	service := NewMultiProviderPullRequestsService()

	assert.NotNil(t, service)
	assert.NotNil(t, service.providers)
	assert.NotNil(t, service.cache)

	// Verify all three providers are registered
	_, githubOK := service.providers["github"]
	assert.True(t, githubOK, "github provider should be registered")

	_, gitlabOK := service.providers["gitlab"]
	assert.True(t, gitlabOK, "gitlab provider should be registered")

	_, bitbucketOK := service.providers["bitbucket"]
	assert.True(t, bitbucketOK, "bitbucket provider should be registered")

	assert.Equal(t, 3, len(service.providers), "should have exactly 3 providers registered")
}

func TestMultiProviderPullRequestsService_GetCache(t *testing.T) {
	service := NewMultiProviderPullRequestsService()

	cache := service.GetCache()
	assert.NotNil(t, cache, "cache should not be nil")
}

func requireUnsupported(t *testing.T, err error, gitProvider, operation string) {
	t.Helper()

	require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
	assert.EqualError(t, err, "provider "+gitProvider+" does not support "+operation+": unsupported operation")
}

func TestMultiProviderPullRequestsService_UnsupportedProvider(t *testing.T) {
	for _, gitProvider := range []string{"unknown", "", "azure", "gerrit"} {
		t.Run(gitProvider, func(t *testing.T) {
			service := NewMultiProviderPullRequestsService()

			result, err := service.ListPullRequests(
				context.Background(),
				"owner",
				"repo",
				krci.GitServerSettings{GitProvider: gitProvider},
				defaultOpts(),
			)

			requireUnsupported(t, err, gitProvider, "listPullRequests")
			assert.Nil(t, result)
		})
	}
}

func TestMultiProviderPullRequestsService_CheckListPullRequests(t *testing.T) {
	service := NewMultiProviderPullRequestsService()

	for _, provider := range []string{"github", "gitlab", "bitbucket"} {
		assert.NoError(t, service.CheckListPullRequests(provider), provider)
	}

	requireUnsupported(t, service.CheckListPullRequests("gerrit"), "gerrit", "listPullRequests")
}
