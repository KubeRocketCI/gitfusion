package pipelines

import (
	"context"
	"slices"
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

func requireUnsupported(t *testing.T, err error, gitProvider, operation string) {
	t.Helper()

	require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
	assert.EqualError(t, err, "provider "+gitProvider+" does not support "+operation+": unsupported operation")
}

func TestMultiProviderPipelineService_TriggerPipeline_UnsupportedProviderReturnsUnsupportedOperation(t *testing.T) {
	// GitHub and Bitbucket are registered but do not implement PipelineTrigger.
	for _, gitProvider := range []string{"unknown", "gerrit", "", "github", "bitbucket"} {
		t.Run(gitProvider, func(t *testing.T) {
			service := NewMultiProviderPipelineService()

			result, err := service.TriggerPipeline(
				context.Background(),
				"test-project",
				"main",
				nil,
				krci.GitServerSettings{GitProvider: gitProvider},
			)

			requireUnsupported(t, err, gitProvider, "triggerPipeline")
			assert.Nil(t, result)
		})
	}
}

func TestMultiProviderPipelineService_GetCache(t *testing.T) {
	service := NewMultiProviderPipelineService()

	cache := service.GetCache()
	assert.NotNil(t, cache, "cache should not be nil")
}

func TestMultiProviderPipelineService_ListPipelines_UnknownProviderReturnsUnsupportedOperation(t *testing.T) {
	for _, gitProvider := range []string{"unknown", "gerrit"} {
		t.Run(gitProvider, func(t *testing.T) {
			service := NewMultiProviderPipelineService()

			result, err := service.ListPipelines(
				context.Background(),
				"test-project",
				krci.GitServerSettings{GitProvider: gitProvider},
				models.PipelineListOptions{Page: 1, PerPage: 20},
			)

			requireUnsupported(t, err, gitProvider, "listPipelines")
			assert.Nil(t, result)
		})
	}
}

func TestMultiProviderPipelineService_ListPipelineJobs_UnsupportedProviderReturnsUnsupportedOperation(t *testing.T) {
	// GitHub is a registered provider but does not implement PipelineJobsProvider.
	for _, gitProvider := range []string{"unknown", "gerrit", "", "github"} {
		t.Run(gitProvider, func(t *testing.T) {
			service := NewMultiProviderPipelineService()

			result, err := service.ListPipelineJobs(
				context.Background(),
				"test-project",
				42,
				krci.GitServerSettings{GitProvider: gitProvider},
			)

			requireUnsupported(t, err, gitProvider, "listPipelineJobs")
			assert.Nil(t, result)
		})
	}
}

func TestMultiProviderPipelineService_GetJobTrace_UnsupportedProviderReturnsUnsupportedOperation(t *testing.T) {
	for _, gitProvider := range []string{"unknown", "gerrit", "github", "bitbucket"} {
		t.Run(gitProvider, func(t *testing.T) {
			service := NewMultiProviderPipelineService()

			content, truncated, err := service.GetJobTrace(
				context.Background(),
				"test-project",
				42,
				krci.GitServerSettings{GitProvider: gitProvider},
			)

			requireUnsupported(t, err, gitProvider, "getPipelineJobTrace")
			assert.Empty(t, content)
			assert.False(t, truncated)
		})
	}
}

func TestMultiProviderPipelineService_Checks(t *testing.T) {
	service := NewMultiProviderPipelineService()

	tests := []struct {
		name      string
		check     func(string) error
		operation string
		supported []string
	}{
		{"trigger", service.CheckTriggerPipeline, "triggerPipeline", []string{"gitlab"}},
		{"list pipelines", service.CheckListPipelines, "listPipelines", []string{"github", "gitlab", "bitbucket"}},
		{"list jobs", service.CheckListPipelineJobs, "listPipelineJobs", []string{"gitlab"}},
		{"job trace", service.CheckGetPipelineJobTrace, "getPipelineJobTrace", []string{"gitlab"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, gitProvider := range []string{"github", "gitlab", "bitbucket", "gerrit"} {
				err := tt.check(gitProvider)

				if slices.Contains(tt.supported, gitProvider) {
					assert.NoError(t, err, gitProvider)

					continue
				}

				requireUnsupported(t, err, gitProvider, tt.operation)
			}
		})
	}
}

type fakeTriggerProvider struct {
	fakeJobsProvider

	gotProject string
	gotRef     string
	response   *models.PipelineResponse
}

func (f *fakeTriggerProvider) TriggerPipeline(
	_ context.Context, project, ref string, _ []models.PipelineVariable, _ krci.GitServerSettings,
) (*models.PipelineResponse, error) {
	f.gotProject = project
	f.gotRef = ref

	return f.response, nil
}

func TestMultiProviderPipelineService_TriggerPipeline_DispatchesToTriggerProvider(t *testing.T) {
	want := &models.PipelineResponse{}
	fake := &fakeTriggerProvider{response: want}
	service := NewMultiProviderPipelineService()
	service.providers["gitlab"] = fake

	got, err := service.TriggerPipeline(
		context.Background(), "test-project", "main", nil, krci.GitServerSettings{GitProvider: "gitlab"},
	)

	require.NoError(t, err)
	assert.Same(t, want, got)
	assert.Equal(t, "test-project", fake.gotProject)
	assert.Equal(t, "main", fake.gotRef)
}
