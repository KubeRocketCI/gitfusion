package api

import (
	"context"
	"fmt"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
)

// CacheHandler handles cache management requests.
type CacheHandler struct {
	cacheManager *cache.Manager
}

// NewCacheHandler creates a new CacheHandler.
func NewCacheHandler(cacheManager *cache.Manager) *CacheHandler {
	return &CacheHandler{
		cacheManager: cacheManager,
	}
}

// InvalidateCache implements api.StrictServerInterface.
func (c *CacheHandler) InvalidateCache(
	ctx context.Context,
	request InvalidateCacheRequestObject,
) (InvalidateCacheResponseObject, error) {
	endpoint := string(request.Params.Endpoint)

	if err := c.cacheManager.InvalidateCache(endpoint); err != nil {
		return nil, err
	}

	return InvalidateCache200JSONResponse{
		Message:  fmt.Sprintf("Cache for endpoint '%s' has been successfully invalidated", endpoint),
		Endpoint: endpoint,
	}, nil
}
