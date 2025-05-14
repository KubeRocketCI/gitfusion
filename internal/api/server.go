package api

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/services"
	codebaseApi "github.com/epam/edp-codebase-operator/v2/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var _ StrictServerInterface = (*Server)(nil)

// Server is the main server struct that implements the StrictServerInterface.
type Server struct {
	gitHubRepositoryHandler *GitHubRepositoryHandler
	gitlabRepositoryHandler *GitlabRepositoryHandler
}

// NewServer creates a new Server instance.
func NewServer(
	gitHubRepositoryHandler *GitHubRepositoryHandler,
	gitlabRepositoryHandler *GitlabRepositoryHandler,
) *Server {
	return &Server{
		gitHubRepositoryHandler: gitHubRepositoryHandler,
		gitlabRepositoryHandler: gitlabRepositoryHandler,
	}
}

// GetGitlabRepository implements StrictServerInterface.
func (r *Server) GetGitlabRepository(
	ctx context.Context,
	request GetGitlabRepositoryRequestObject,
) (GetGitlabRepositoryResponseObject, error) {
	return r.gitlabRepositoryHandler.GetGitlabRepository(ctx, request)
}

// ListGitlabRepositories implements StrictServerInterface.
func (r *Server) ListGitlabRepositories(
	ctx context.Context,
	request ListGitlabRepositoriesRequestObject,
) (ListGitlabRepositoriesResponseObject, error) {
	return r.gitlabRepositoryHandler.ListGitlabRepositories(ctx, request)
}

// GetGitHubRepository implements StrictServerInterface.
func (r *Server) GetGitHubRepository(
	ctx context.Context,
	request GetGitHubRepositoryRequestObject,
) (GetGitHubRepositoryResponseObject, error) {
	return r.gitHubRepositoryHandler.GetGitHubRepository(ctx, request)
}

// ListGitHubRepositories implements StrictServerInterface.
func (r *Server) ListGitHubRepositories(
	ctx context.Context,
	request ListGitHubRepositoriesRequestObject,
) (ListGitHubRepositoriesResponseObject, error) {
	return r.gitHubRepositoryHandler.ListGitHubRepositories(ctx, request)
}

func BuildHandler(conf Config) (ServerInterface, error) {
	k8sCl, err := initk8sClient()
	if err != nil {
		return nil, err
	}

	gitServerService := services.NewGitServerService(k8sCl, conf.Namespace)

	return NewStrictHandlerWithOptions(
		NewServer(
			NewGitHubRepositoryHandler(
				services.NewRepositoriesService(
					services.NewGitHubService(),
					gitServerService,
				),
			),
			NewGitlabRepositoryHandler(
				services.NewRepositoriesService(
					services.NewGitlabService(),
					gitServerService,
				),
			),
		),
		[]StrictMiddlewareFunc{},
		StrictHTTPServerOptions{},
	), nil
}

func initk8sClient() (client.Client, error) {
	k8sCfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	if err = corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err = codebaseApi.AddToScheme(scheme); err != nil {
		return nil, err
	}

	k8sCl, err := client.New(k8sCfg, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	return k8sCl, nil
}
