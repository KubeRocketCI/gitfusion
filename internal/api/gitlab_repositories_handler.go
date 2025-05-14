package api

import (
	"context"
	"errors"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services"
)

// GitHubRepositoryHandler handles requests related to GitHub repositories.
type GitlabRepositoryHandler struct {
	repositoriesService *services.RepositoriesService
}

// NewGitlabRepositoryHandler creates a new GitlabRepositoryHandler.
func NewGitlabRepositoryHandler(repositoriesService *services.RepositoriesService) *GitlabRepositoryHandler {
	return &GitlabRepositoryHandler{
		repositoriesService: repositoriesService,
	}
}

// GetGitlabRepository implements StrictServerInterface.
func (r *GitlabRepositoryHandler) GetGitlabRepository(
	ctx context.Context,
	request GetGitlabRepositoryRequestObject,
) (GetGitlabRepositoryResponseObject, error) {
	repo, err := r.repositoriesService.GetRepository(ctx, request.GitServer, request.Owner, request.Repo)
	if err != nil {
		return r.gitlabErrResponse(err), nil
	}

	return GetGitlabRepository200JSONResponse(*repo), nil
}

// ListGitlabRepositories implements StrictServerInterface.
func (r *GitlabRepositoryHandler) ListGitlabRepositories(
	ctx context.Context,
	request ListGitlabRepositoriesRequestObject,
) (ListGitlabRepositoriesResponseObject, error) {
	repositories, err := r.repositoriesService.ListOrganizationsRepositories(
		ctx,
		request.GitServer,
		request.Org,
		getListOptions(request.Params.Page, request.Params.PerPage),
	)
	if err != nil {
		return ListGitlabRepositories400JSONResponse{
			Message: err.Error(),
			Code:    "bad_request",
		}, nil
	}

	return ListGitlabRepositories200JSONResponse{
		Repositories: repositories,
		Pagination:   models.Pagination{}, // TODO: implement pagination after all rit providers are implemented
	}, nil
}

func (r *GitlabRepositoryHandler) gitlabErrResponse(err error) GetGitlabRepositoryResponseObject {
	if err == nil {
		return GetGitlabRepository200JSONResponse{}
	}

	if errors.Is(err, gferrors.ErrNotFound) {
		return GetGitlabRepository404JSONResponse{
			Message: err.Error(),
			Code:    "not_found",
		}
	}

	return GetGitlabRepository400JSONResponse{
		Message: err.Error(),
		Code:    "bad_request",
	}
}
