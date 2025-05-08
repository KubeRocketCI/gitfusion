package api

import (
	"context"
	"errors"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services"
)

// GetGitHubRepository implements api.StrictServerInterface.
func (r *Server) GetGitHubRepository(
	ctx context.Context,
	request GetGitHubRepositoryRequestObject,
) (GetGitHubRepositoryResponseObject, error) {
	repo, err := r.repositoriesService.GetRepository(ctx, request.GitServer, request.Owner, request.Repo)
	if err != nil {
		return r.errResponse(err), nil
	}

	return GetGitHubRepository200JSONResponse(*repo), nil
}

// ListGitHubRepositories implements api.StrictServerInterface.
func (r *Server) ListGitHubRepositories(
	ctx context.Context,
	request ListGitHubRepositoriesRequestObject,
) (ListGitHubRepositoriesResponseObject, error) {
	repositories, err := r.repositoriesService.ListOrganizationsRepositories(
		ctx,
		request.GitServer,
		request.Org,
		getListOptions(request.Params.Pagination),
	)
	if err != nil {
		return ListGitHubRepositories400JSONResponse{
			Message: err.Error(),
			Code:    "bad_request",
		}, nil
	}

	return ListGitHubRepositories200JSONResponse{
		Repositories: repositories,
		Pagination:   models.Pagination{}, // TODO: implement pagination after all rit providers are implemented
	}, nil
}

func getListOptions(pagination *models.PaginationParameters) services.ListOptions {
	listOptions := services.ListOptions{}

	if pagination != nil {
		if pagination.PerPage != nil {
			listOptions.PerPage = pagination.PerPage
		}

		if pagination.Page != nil {
			listOptions.Page = pagination.Page
		}
	}

	return listOptions
}

func (r *Server) errResponse(err error) GetGitHubRepositoryResponseObject {
	if err == nil {
		return GetGitHubRepository200JSONResponse{}
	}

	if errors.Is(err, gferrors.ErrNotFound) {
		return GetGitHubRepository404JSONResponse{
			Message: err.Error(),
			Code:    "not_found",
		}
	}

	return GetGitHubRepository400JSONResponse{
		Message: err.Error(),
		Code:    "bad_request",
	}
}
