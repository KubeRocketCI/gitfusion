package api

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	"github.com/KubeRocketCI/gitfusion/internal/services/branches"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	"github.com/KubeRocketCI/gitfusion/internal/services/organizations"
	"github.com/KubeRocketCI/gitfusion/internal/services/repositories"
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
	repositoryHandler   *RepositoryHandler
	organizationHandler *OrganizationHandler
	branchHandler       *BranchHandler
	cacheHandler        *CacheHandler
}

// NewServer creates a new Server instance.
func NewServer(
	repositoryHandler *RepositoryHandler,
	organizationHandler *OrganizationHandler,
	branchHandler *BranchHandler,
	cacheHandler *CacheHandler,
) *Server {
	return &Server{
		repositoryHandler:   repositoryHandler,
		organizationHandler: organizationHandler,
		branchHandler:       branchHandler,
		cacheHandler:        cacheHandler,
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

// ListUserOrganizations implements StrictServerInterface.
func (s *Server) ListUserOrganizations(
	ctx context.Context,
	request ListUserOrganizationsRequestObject,
) (ListUserOrganizationsResponseObject, error) {
	return s.organizationHandler.ListUserOrganizations(ctx, request)
}

// ListBranches implements StrictServerInterface.
func (s *Server) ListBranches(
	ctx context.Context,
	request ListBranchesRequestObject,
) (ListBranchesResponseObject, error) {
	return s.branchHandler.ListBranches(ctx, request)
}

// InvalidateCache implements StrictServerInterface.
func (s *Server) InvalidateCache(
	ctx context.Context,
	request InvalidateCacheRequestObject,
) (InvalidateCacheResponseObject, error) {
	return s.cacheHandler.InvalidateCache(ctx, request)
}

func BuildHandler(conf Config) (ServerInterface, error) {
	k8sCl, err := initk8sClient()
	if err != nil {
		return nil, err
	}

	gitServerService := krci.NewGitServerService(k8sCl, conf.Namespace)

	// Create multi-provider services
	repoMultiProvider := repositories.NewMultiProviderRepositoryService()
	orgMultiProvider := organizations.NewMultiProviderOrganizationsService(gitServerService)
	branchesMultiProvider := branches.NewMultiProviderBranchesService()

	// Create high-level services
	repoSvc := repositories.NewRepositoriesService(repoMultiProvider, gitServerService)
	orgSvc := organizations.NewOrganizationsService(orgMultiProvider, gitServerService)
	branchesSvc := branches.NewBranchesService(branchesMultiProvider, gitServerService)

	// Create cache manager with access to all cache instances
	cacheManager := cache.NewManager(
		repoSvc.GetProvider().GetCache(),
		orgSvc.GetProvider().GetCache(),
		branchesSvc.GetProvider().GetCache(),
	)

	// Create handlers
	branchHandler := NewBranchHandler(branchesSvc)
	cacheHandler := NewCacheHandler(cacheManager)

	return NewStrictHandlerWithOptions(
		NewServer(
			NewRepositoryHandler(repoSvc),
			NewOrganizationHandler(orgSvc),
			branchHandler,
			cacheHandler,
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
