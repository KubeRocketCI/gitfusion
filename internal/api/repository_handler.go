package api

import (
	"context"
	"errors"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/repositories"
)

// RepositoryHandler handles requests related to repositories (all providers).
type RepositoryHandler struct {
	repositoriesService *repositories.RepositoriesService
}

// NewRepositoryHandler creates a new RepositoryHandler.
func NewRepositoryHandler(repositoriesService *repositories.RepositoriesService) *RepositoryHandler {
	return &RepositoryHandler{
		repositoriesService: repositoriesService,
	}
}

// GetRepository implements api.StrictServerInterface.
func (r *RepositoryHandler) GetRepository(
	ctx context.Context,
	request GetRepositoryRequestObject,
) (GetRepositoryResponseObject, error) {
	repo, err := r.repositoriesService.GetRepository(
		ctx,
		request.Params.GitServer,
		request.Params.Owner,
		request.Params.RepoName,
	)
	if err != nil {
		return r.errResponse(err), nil
	}

	return GetRepository200JSONResponse(*repo), nil
}

// ListRepositories implements api.StrictServerInterface.
func (r *RepositoryHandler) ListRepositories(
	ctx context.Context,
	request ListRepositoriesRequestObject,
) (ListRepositoriesResponseObject, error) {
	repositories, err := r.repositoriesService.ListRepositories(
		ctx,
		request.Params.GitServer,
		request.Params.Owner,
		r.getListOptions(request),
	)
	if err != nil {
		return ListRepositories400JSONResponse{
			Message: err.Error(),
			Code:    "bad_request",
		}, nil
	}

	return ListRepositories200JSONResponse{
		Data: repositories,
	}, nil
}

func (r *RepositoryHandler) errResponse(err error) GetRepositoryResponseObject {
	if err == nil {
		return GetRepository200JSONResponse{}
	}

	if errors.Is(err, gferrors.ErrNotFound) {
		return GetRepository404JSONResponse{
			Message: err.Error(),
			Code:    "not_found",
		}
	}

	return GetRepository400JSONResponse{
		Message: err.Error(),
		Code:    "bad_request",
	}
}

func (r *RepositoryHandler) getListOptions(
	request ListRepositoriesRequestObject,
) models.ListOptions {
	return models.ListOptions{
		Name: request.Params.RepoName,
	}
}
