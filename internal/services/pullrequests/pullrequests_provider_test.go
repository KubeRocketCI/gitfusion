package pullrequests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

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

func TestMultiProviderPullRequestsService_UnsupportedProvider(t *testing.T) {
	tests := []struct {
		name           string
		gitProvider    string
		expectedErrMsg string
	}{
		{
			name:           "unknown provider",
			gitProvider:    "unknown",
			expectedErrMsg: "unsupported provider: unknown",
		},
		{
			name:           "empty provider",
			gitProvider:    "",
			expectedErrMsg: "unsupported provider: ",
		},
		{
			name:           "azure provider not supported",
			gitProvider:    "azure",
			expectedErrMsg: "unsupported provider: azure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewMultiProviderPullRequestsService()

			result, err := service.ListPullRequests(
				context.Background(),
				"owner",
				"repo",
				krci.GitServerSettings{GitProvider: tt.gitProvider},
				defaultOpts(),
			)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
		})
	}
}
