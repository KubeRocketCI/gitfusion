package pipelines

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

func TestNewMultiProviderPipelineService(t *testing.T) {
	service := NewMultiProviderPipelineService()

	assert.NotNil(t, service)
	assert.NotNil(t, service.providers)

	// Verify all providers are registered
	_, ok := service.providers["gitlab"]
	assert.True(t, ok, "gitlab provider should be registered")

	_, ok = service.providers["github"]
	assert.True(t, ok, "github provider should be registered")

	_, ok = service.providers["bitbucket"]
	assert.True(t, ok, "bitbucket provider should be registered")

	// Verify only expected providers are registered
	assert.Equal(t, 3, len(service.providers), "should have exactly 3 providers registered (gitlab, github, bitbucket)")
}

func TestMultiProviderPipelineService_UnsupportedProvider(t *testing.T) {
	service := NewMultiProviderPipelineService()

	// Test unsupported providers
	unsupported := []string{"unknown", ""}

	for _, provider := range unsupported {
		t.Run(provider, func(t *testing.T) {
			_, ok := service.providers[provider]
			assert.False(t, ok, "provider %s should not be registered yet", provider)
		})
	}
}

func TestMultiProviderPipelineService_TriggerPipeline_UnknownProviderReturnsUnsupportedOperation(t *testing.T) {
	tests := []struct {
		name           string
		gitProvider    string
		expectedErrMsg string
	}{
		{
			name:           "unknown provider",
			gitProvider:    "unknown",
			expectedErrMsg: "unsupported provider unknown",
		},
		{
			name:           "gerrit provider",
			gitProvider:    "gerrit",
			expectedErrMsg: "unsupported provider gerrit",
		},
		{
			name:           "empty provider",
			gitProvider:    "",
			expectedErrMsg: "unsupported provider ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewMultiProviderPipelineService()

			result, err := service.TriggerPipeline(
				context.Background(),
				"test-project",
				"main",
				nil,
				krci.GitServerSettings{GitProvider: tt.gitProvider},
			)

			require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
		})
	}
}

func TestMultiProviderPipelineService_GetCache(t *testing.T) {
	service := NewMultiProviderPipelineService()

	cache := service.GetCache()
	assert.NotNil(t, cache, "cache should not be nil")
}

func TestMultiProviderPipelineService_ListPipelines_UnknownProviderReturnsUnsupportedOperation(t *testing.T) {
	tests := []struct {
		name           string
		gitProvider    string
		expectedErrMsg string
	}{
		{
			name:           "unknown provider",
			gitProvider:    "unknown",
			expectedErrMsg: "unsupported provider unknown",
		},
		{
			name:           "gerrit provider",
			gitProvider:    "gerrit",
			expectedErrMsg: "unsupported provider gerrit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewMultiProviderPipelineService()

			result, err := service.ListPipelines(
				context.Background(),
				"test-project",
				krci.GitServerSettings{GitProvider: tt.gitProvider},
				models.PipelineListOptions{Page: 1, PerPage: 20},
			)

			require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
		})
	}
}

func TestMultiProviderPipelineService_ListPipelineJobs_UnknownProviderReturnsUnsupportedOperation(t *testing.T) {
	tests := []struct {
		name        string
		gitProvider string
	}{
		{name: "unknown provider", gitProvider: "unknown"},
		{name: "gerrit provider", gitProvider: "gerrit"},
		{name: "empty provider", gitProvider: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewMultiProviderPipelineService()

			result, err := service.ListPipelineJobs(
				context.Background(),
				"test-project",
				42,
				krci.GitServerSettings{GitProvider: tt.gitProvider},
			)

			require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "unsupported provider "+tt.gitProvider)
		})
	}
}

func TestMultiProviderPipelineService_ListPipelineJobs_GitHubReturnsUnsupportedOperation(t *testing.T) {
	// GitHub is a registered provider but does not implement PipelineJobsProvider.
	service := NewMultiProviderPipelineService()

	result, err := service.ListPipelineJobs(
		context.Background(),
		"test-project",
		1,
		krci.GitServerSettings{GitProvider: "github"},
	)

	require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "provider github does not support pipeline jobs")
}
