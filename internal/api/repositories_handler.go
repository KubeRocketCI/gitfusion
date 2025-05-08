package api

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/services"
)

var _ StrictServerInterface = (*Server)(nil)

type Server struct {
	repositoriesService *services.RepositoriesService
}

func NewServer(repositoriesService *services.RepositoriesService) *Server {
	return &Server{
		repositoriesService: repositoriesService,
	}
}

// GetGitHubRepository implements api.StrictServerInterface.
func (r *Server) GetGitHubRepository(ctx context.Context, request GetGitHubRepositoryRequestObject) (GetGitHubRepositoryResponseObject, error) {
	_, _ = r.repositoriesService.GetRepository(ctx, request.GitServer, request.Id)

	return GetGitHubRepository200JSONResponse{}, nil
}

// ListGitHubRepositories implements api.StrictServerInterface.
func (r *Server) ListGitHubRepositories(ctx context.Context, request ListGitHubRepositoriesRequestObject) (ListGitHubRepositoriesResponseObject, error) {
	panic("unimplemented")
}
