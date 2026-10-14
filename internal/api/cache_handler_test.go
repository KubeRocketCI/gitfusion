package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	"github.com/KubeRocketCI/gitfusion/internal/models"
)

func newTestCacheHandler() *CacheHandler {
	return NewCacheHandler(cache.NewManager(
		cache.NewRepositoryCache(),
		cache.NewOrganizationCache(),
		cache.NewBranchCache(),
		cache.NewPullRequestCache(),
		cache.NewPipelineCache(),
		cache.NewPipelineJobsCache(),
		cache.NewPipelineJobTraceCache(),
	))
}

func invalidateCacheRequest(endpoint models.InvalidateCacheParamsEndpoint) InvalidateCacheRequestObject {
	return InvalidateCacheRequestObject{Params: models.InvalidateCacheParams{Endpoint: endpoint}}
}

func TestCacheHandlerInvalidateCacheKnownEndpoint(t *testing.T) {
	resp, err := newTestCacheHandler().InvalidateCache(context.Background(), invalidateCacheRequest(models.Repositories))

	require.NoError(t, err)
	assert.Equal(t, InvalidateCache200JSONResponse{
		Endpoint: "repositories",
		Message:  "Cache for endpoint 'repositories' has been successfully invalidated",
	}, resp)
}

func TestCacheHandlerInvalidateCacheUnknownEndpointReturnsInvalidParamError(t *testing.T) {
	resp, err := newTestCacheHandler().InvalidateCache(context.Background(), invalidateCacheRequest("foo"))

	assert.Nil(t, resp)
	requireInvalidParam(t, err, "endpoint")
}
