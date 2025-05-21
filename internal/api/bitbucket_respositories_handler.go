package api

import (
	"context"
	"errors"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/services"
)

type BitbucketRepositoryHandler struct {
	repositoriesService *services.RepositoriesService
}

// NewBitbucketRepositoryHandler creates a new BitbucketRepositoryHandler.
func NewBitbucketRepositoryHandler(repositoriesService *services.RepositoriesService) *BitbucketRepositoryHandler {
	return &BitbucketRepositoryHandler{
		repositoriesService: repositoriesService,
	}
}

// GetBitbucketRepository implements api.StrictServerInterface.
func (r *BitbucketRepositoryHandler) GetBitbucketRepository(
	ctx context.Context,
	request GetBitbucketRepositoryRequestObject,
) (GetBitbucketRepositoryResponseObject, error) {
	repo, err := r.repositoriesService.GetRepository(ctx, request.GitServer, request.Owner, request.Repo)
	if err != nil {
		return r.errResponse(err), nil
	}

	return GetBitbucketRepository200JSONResponse(*repo), nil
}

// ListBitbucketRepositories implements api.StrictServerInterface.
func (r *BitbucketRepositoryHandler) ListBitbucketRepositories(
	ctx context.Context,
	request ListBitbucketRepositoriesRequestObject,
) (ListBitbucketRepositoriesResponseObject, error) {
	repositories, err := r.repositoriesService.ListRepositories(
		ctx,
		request.GitServer,
		request.Owner,
		r.getListOptions(request),
	)
	if err != nil {
		return ListBitbucketRepositories400JSONResponse{
			Message: err.Error(),
			Code:    "bad_request",
		}, nil
	}

	return ListBitbucketRepositories200JSONResponse{
		Data: repositories,
	}, nil
}

func (r *BitbucketRepositoryHandler) getListOptions(
	request ListBitbucketRepositoriesRequestObject,
) services.ListOptions {
	return services.ListOptions{
		Name: request.Params.RepoName,
	}
}

func (r *BitbucketRepositoryHandler) errResponse(err error) GetBitbucketRepositoryResponseObject {
	if err == nil {
		return GetBitbucketRepository200JSONResponse{}
	}

	if errors.Is(err, gferrors.ErrNotFound) {
		return GetBitbucketRepository404JSONResponse{
			Message: err.Error(),
			Code:    "not_found",
		}
	}

	return GetBitbucketRepository400JSONResponse{
		Message: err.Error(),
		Code:    "bad_request",
	}
}
