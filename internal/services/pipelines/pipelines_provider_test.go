package pipelines

import (
	"context"
	"testing"

	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	"github.com/stretchr/testify/assert"
)

func TestNewMultiProviderPipelineService(t *testing.T) {
	service := NewMultiProviderPipelineService()

	assert.NotNil(t, service)
	assert.NotNil(t, service.providers)

	// Verify gitlab provider is registered
	_, ok := service.providers["gitlab"]
	assert.True(t, ok, "gitlab provider should be registered")

	// Verify only expected providers are registered
	assert.Equal(t, 1, len(service.providers), "should have exactly 1 provider registered (gitlab)")
}

func TestMultiProviderPipelineService_UnsupportedProvider(t *testing.T) {
	service := NewMultiProviderPipelineService()

	// Test unsupported providers
	unsupported := []string{"github", "bitbucket", "unknown", ""}

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
			name:           "github not supported yet",
			gitProvider:    "github",
			expectedErrMsg: "unsupported provider: github",
		},
		{
			name:           "bitbucket not supported yet",
			gitProvider:    "bitbucket",
			expectedErrMsg: "unsupported provider: bitbucket",
		},
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
		})
	}
}
