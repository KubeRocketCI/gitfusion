package pipelines

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPipelinesService(t *testing.T) {
	t.Run("creates service with valid dependencies", func(t *testing.T) {
		multiProvider := NewMultiProviderPipelineService()
		assert.NotNil(t, multiProvider)
	})
}

func TestPipelinesService_GetProvider(t *testing.T) {
	multiProvider := NewMultiProviderPipelineService()
	service := NewPipelinesService(multiProvider, nil)

	provider := service.GetProvider()
	assert.NotNil(t, provider)
	assert.Equal(t, multiProvider, provider)
}

// Integration tests for PipelinesService require a kubernetes cluster with GitServer CRs
// and are skipped in unit tests. See internal/services/gitlab/gitlab_test.go for similar pattern.
func TestPipelinesService_Integration(t *testing.T) {
	t.Skip("Integration test - requires Kubernetes cluster with GitServer CRs")
}
