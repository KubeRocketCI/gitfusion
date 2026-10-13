package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
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

// errResponse must only be called when err is not nil.
func (c *CacheHandler) errResponse(err error) InvalidateCacheResponseObject {
	if errors.Is(err, gferrors.ErrBadRequest) {
		return InvalidateCache400JSONResponse{
			Code:    "invalid_endpoint",
			Message: err.Error(),
		}
	}

	return InvalidateCache500JSONResponse{
		Code:    "internal_error",
		Message: err.Error(),
	}
}
