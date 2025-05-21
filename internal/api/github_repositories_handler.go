package api

import (
	"context"
	"errors"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/services"
)

// GitHubRepositoryHandler handles requests related to GitHub repositories.
type GitHubRepositoryHandler struct {
	repositoriesService *services.RepositoriesService
}

// NewGitHubRepositoryHandler creates a new GitHubRepositoryHandler.
func NewGitHubRepositoryHandler(repositoriesService *services.RepositoriesService) *GitHubRepositoryHandler {
	return &GitHubRepositoryHandler{
		repositoriesService: repositoriesService,
	}
}

// GetGitHubRepository implements api.StrictServerInterface.
func (r *GitHubRepositoryHandler) GetGitHubRepository(
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
func (r *GitHubRepositoryHandler) ListGitHubRepositories(
	ctx context.Context,
	request ListGitHubRepositoriesRequestObject,
) (ListGitHubRepositoriesResponseObject, error) {
	repositories, err := r.repositoriesService.ListRepositories(
		ctx,
		request.GitServer,
		request.Owner,
		r.getListOptions(request),
	)
	if err != nil {
		return ListGitHubRepositories400JSONResponse{
			Message: err.Error(),
			Code:    "bad_request",
		}, nil
	}

	return ListGitHubRepositories200JSONResponse{
		Data: repositories,
	}, nil
}

func (r *GitHubRepositoryHandler) errResponse(err error) GetGitHubRepositoryResponseObject {
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

func (r *GitHubRepositoryHandler) getListOptions(
	request ListGitHubRepositoriesRequestObject,
) services.ListOptions {
	return services.ListOptions{
		Name: request.Params.RepoName,
	}
}
