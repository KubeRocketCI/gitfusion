package api

import (
	"context"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	codebaseApi "github.com/epam/edp-codebase-operator/v2/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	"github.com/KubeRocketCI/gitfusion/internal/services/branches"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	"github.com/KubeRocketCI/gitfusion/internal/services/organizations"
	"github.com/KubeRocketCI/gitfusion/internal/services/pipelines"
	"github.com/KubeRocketCI/gitfusion/internal/services/pullrequests"
	"github.com/KubeRocketCI/gitfusion/internal/services/repositories"
)

var _ StrictServerInterface = (*Server)(nil)

// Server is the main server struct that implements the StrictServerInterface.
type Server struct {
	repositoryHandler   *RepositoryHandler
	organizationHandler *OrganizationHandler
	branchHandler       *BranchHandler
	cacheHandler        *CacheHandler
	pipelineHandler     *PipelineHandler
	pullRequestHandler  *PullRequestHandler
}

// NewServer creates a new Server instance.
func NewServer(
	repositoryHandler *RepositoryHandler,
	organizationHandler *OrganizationHandler,
	branchHandler *BranchHandler,
	cacheHandler *CacheHandler,
	pipelineHandler *PipelineHandler,
	pullRequestHandler *PullRequestHandler,
) *Server {
	return &Server{
		repositoryHandler:   repositoryHandler,
		organizationHandler: organizationHandler,
		branchHandler:       branchHandler,
		cacheHandler:        cacheHandler,
		pipelineHandler:     pipelineHandler,
		pullRequestHandler:  pullRequestHandler,
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

// ListPullRequests implements StrictServerInterface.
func (s *Server) ListPullRequests(
	ctx context.Context,
	request ListPullRequestsRequestObject,
) (ListPullRequestsResponseObject, error) {
	return s.pullRequestHandler.ListPullRequests(ctx, request)
}

// TriggerPipeline implements StrictServerInterface.
func (s *Server) TriggerPipeline(
	ctx context.Context,
	request TriggerPipelineRequestObject,
) (TriggerPipelineResponseObject, error) {
	return s.pipelineHandler.TriggerPipeline(ctx, request)
}

// ListPipelines implements StrictServerInterface.
func (s *Server) ListPipelines(
	ctx context.Context,
	request ListPipelinesRequestObject,
) (ListPipelinesResponseObject, error) {
	return s.pipelineHandler.ListPipelines(ctx, request)
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
	pipelinesMultiProvider := pipelines.NewMultiProviderPipelineService()
	pullRequestsMultiProvider := pullrequests.NewMultiProviderPullRequestsService()

	// Create high-level services
	repoSvc := repositories.NewRepositoriesService(repoMultiProvider, gitServerService)
	orgSvc := organizations.NewOrganizationsService(orgMultiProvider, gitServerService)
	branchesSvc := branches.NewBranchesService(branchesMultiProvider, gitServerService)
	pipelinesSvc := pipelines.NewPipelinesService(pipelinesMultiProvider, gitServerService)
	pullRequestsSvc := pullrequests.NewPullRequestsService(pullRequestsMultiProvider, gitServerService)

	// Create cache manager with access to all cache instances
	cacheManager := cache.NewManager(
		repoSvc.GetProvider().GetCache(),
		orgSvc.GetProvider().GetCache(),
		branchesSvc.GetProvider().GetCache(),
		pullRequestsSvc.GetProvider().GetCache(),
		pipelinesSvc.GetProvider().GetCache(),
	)

	// Create handlers
	branchHandler := NewBranchHandler(branchesSvc)
	cacheHandler := NewCacheHandler(cacheManager)
	pipelineHandler := NewPipelineHandler(pipelinesSvc)
	pullRequestHandler := NewPullRequestHandler(pullRequestsSvc)

	return NewStrictHandlerWithOptions(
		NewServer(
			NewRepositoryHandler(repoSvc),
			NewOrganizationHandler(orgSvc),
			branchHandler,
			cacheHandler,
			pipelineHandler,
			pullRequestHandler,
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
