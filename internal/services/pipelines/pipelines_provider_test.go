package pipelines

import (
	"context"
	"errors"
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

func TestMultiProviderPipelineService_TriggerPipeline_UnsupportedProvider(t *testing.T) {
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

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
			assert.True(t, errors.Is(err, gferrors.ErrBadRequest),
				"unsupported provider should map to ErrBadRequest, got: %v", err)
		})
	}
}

func TestMultiProviderPipelineService_GetCache(t *testing.T) {
	service := NewMultiProviderPipelineService()

	cache := service.GetCache()
	assert.NotNil(t, cache, "cache should not be nil")
}

func TestMultiProviderPipelineService_ListPipelines_UnsupportedProvider(t *testing.T) {
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

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
			assert.True(t, errors.Is(err, gferrors.ErrBadRequest),
				"unsupported provider should map to ErrBadRequest, got: %v", err)
		})
	}
}

func TestMultiProviderPipelineService_ListPipelineJobs_UnsupportedProviderReturnsBadRequest(t *testing.T) {
	tests := []struct {
		name        string
		gitProvider string
	}{
		{name: "unknown provider returns ErrBadRequest", gitProvider: "unknown"},
		{name: "empty provider returns ErrBadRequest", gitProvider: ""},
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

			require.Error(t, err)
			assert.Nil(t, result)
			assert.True(t, errors.Is(err, gferrors.ErrBadRequest),
				"expected ErrBadRequest for unsupported provider %q, got: %v", tt.gitProvider, err)
		})
	}
}

func TestMultiProviderPipelineService_ListPipelineJobs_GitHubReturnsBadRequest(t *testing.T) {
	// GitHub is a registered provider but does not implement PipelineJobsProvider.
	service := NewMultiProviderPipelineService()

	result, err := service.ListPipelineJobs(
		context.Background(),
		"test-project",
		1,
		krci.GitServerSettings{GitProvider: "github"},
	)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, gferrors.ErrBadRequest),
		"expected ErrBadRequest for github provider (no jobs support), got: %v", err)
}
