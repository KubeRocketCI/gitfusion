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
		return c.errResponse(err), nil
	}

	return InvalidateCache200JSONResponse{
		Message:  fmt.Sprintf("Cache for endpoint '%s' has been successfully invalidated", endpoint),
		Endpoint: endpoint,
	}, nil
}

// errResponse handles error responses for cache operations.
func (c *CacheHandler) errResponse(err error) InvalidateCacheResponseObject {
	if err == nil {
		return InvalidateCache200JSONResponse{}
	}

	// Check if it's an unsupported endpoint error (validation error)
	if err.Error() == "unsupported endpoint" {
		return InvalidateCache400JSONResponse{
			Code:    "invalid_endpoint",
			Message: err.Error(),
		}
	}

	// Default to 500 internal server error
	return InvalidateCache500JSONResponse{
		Code:    "internal_error",
		Message: err.Error(),
	}
}
