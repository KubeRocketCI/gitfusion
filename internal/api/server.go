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
	repositoryHandler *RepositoryHandler
}

// NewServer creates a new Server instance.
func NewServer(
	repositoryHandler *RepositoryHandler,
) *Server {
	return &Server{
		repositoryHandler: repositoryHandler,
	}
}

// GetRepository implements StrictServerInterface.
func (r *Server) GetRepository(
	ctx context.Context,
	request GetRepositoryRequestObject,
) (GetRepositoryResponseObject, error) {
	return r.repositoryHandler.GetRepository(ctx, request)
}

// ListRepositories implements StrictServerInterface.
func (r *Server) ListRepositories(
	ctx context.Context,
	request ListRepositoriesRequestObject,
) (ListRepositoriesResponseObject, error) {
	return r.repositoryHandler.ListRepositories(ctx, request)
}

func BuildHandler(conf Config) (ServerInterface, error) {
	k8sCl, err := initk8sClient()
	if err != nil {
		return nil, err
	}

	gitServerService := services.NewGitServerService(k8sCl, conf.Namespace)

	return NewStrictHandlerWithOptions(
		NewServer(
			NewRepositoryHandler(
				services.NewRepositoriesService(
					services.NewMultiProviderRepositoryService(), // Dynamically supports GitHub, GitLab, Bitbucket
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
