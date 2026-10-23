package krci

import (
	"context"
	"errors"
	"fmt"
	"time"

	codebaseApi "github.com/epam/edp-codebase-operator/v2/api/v1"
	gitprovider "github.com/epam/edp-codebase-operator/v2/pkg/gitprovider"
	codebaseUtil "github.com/epam/edp-codebase-operator/v2/pkg/util"
	"github.com/viccon/sturdyc"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
)

// cacheTTL bounds how long a rotated token or a changed GitServer stays in use.
const (
	cacheTTL      = 60 * time.Second
	cacheCapacity = 200
)

type GitServerService struct {
	k8sClinet client.Client
	namespace string

	// gitServerCache caches the GitServer CR per name; fetch errors are not cached.
	gitServerCache *sturdyc.Client[gitServer]
	// tokenCache caches the provider token per Secret name; fetch errors are not cached.
	tokenCache *sturdyc.Client[string]
}

type GitServerSettings struct {
	Url           string
	Token         string
	GitProvider   string
	GitServerName string
}

// gitServer is the part of a GitServer CR that is readable without the Secret.
type gitServer struct {
	Name        string
	GitProvider string
	Url         string
	SecretName  string
}

func NewGitServerService(k8sClinet client.Client, namespace string) *GitServerService {
	return &GitServerService{
		k8sClinet:      k8sClinet,
		namespace:      namespace,
		gitServerCache: newCache[gitServer](),
		tokenCache:     newCache[string](),
	}
}

func newCache[T any]() *sturdyc.Client[T] {
	numShards := 8
	evictionPercentage := 10

	return sturdyc.New[T](cacheCapacity, numShards, cacheTTL, evictionPercentage)
}

// Resolve returns the settings of the named GitServer. Order: GitServer CR → check(gitProvider) → Secret + token.
// A GitServer that does not exist yields an error matching gferrors.ErrNotFound; a missing Secret does not.
// An error from check is returned unchanged and the Secret is not read.
func (g *GitServerService) Resolve(
	ctx context.Context,
	name string,
	check func(gitProvider string) error,
) (GitServerSettings, error) {
	gs, err := g.getGitServer(ctx, name)
	if err != nil {
		return GitServerSettings{}, err
	}

	if err = check(gs.GitProvider); err != nil {
		return GitServerSettings{}, err
	}

	token, err := g.getToken(ctx, gs.SecretName)
	if err != nil {
		return GitServerSettings{}, err
	}

	return GitServerSettings{
		Url:           gs.Url,
		Token:         token,
		GitProvider:   gs.GitProvider,
		GitServerName: gs.Name,
	}, nil
}

// GitServerNames lists GitServer names in the namespace and caches each listed GitServer; reads no Secrets.
func (g *GitServerService) GitServerNames(ctx context.Context) ([]string, error) {
	gitServerList := &codebaseApi.GitServerList{}
	if err := g.k8sClinet.List(ctx, gitServerList, client.InNamespace(g.namespace)); err != nil {
		return nil, fmt.Errorf("failed to list GitServers: %w", err)
	}

	names := make([]string, 0, len(gitServerList.Items))

	for i := range gitServerList.Items {
		cr := &gitServerList.Items[i]
		g.gitServerCache.Set(cr.Name, gitServerFromCR(cr))
		names = append(names, cr.Name)
	}

	return names, nil
}

func (g *GitServerService) getGitServer(ctx context.Context, name string) (gitServer, error) {
	return g.gitServerCache.GetOrFetch(ctx, name, func(ctx context.Context) (gitServer, error) {
		cr := &codebaseApi.GitServer{}
		if err := g.k8sClinet.Get(ctx, client.ObjectKey{Name: name, Namespace: g.namespace}, cr); err != nil {
			if apierrors.IsNotFound(err) {
				return gitServer{}, fmt.Errorf("git server %s: %w: %w", name, gferrors.ErrNotFound, err)
			}

			return gitServer{}, err
		}

		return gitServerFromCR(cr), nil
	})
}

func gitServerFromCR(cr *codebaseApi.GitServer) gitServer {
	return gitServer{
		Name:        cr.Name,
		GitProvider: cr.Spec.GitProvider,
		Url:         gitprovider.GetGitProviderAPIURL(cr),
		SecretName:  cr.Spec.NameSshKeySecret,
	}
}

func (g *GitServerService) getToken(ctx context.Context, secretName string) (string, error) {
	return g.tokenCache.GetOrFetch(ctx, secretName, func(ctx context.Context) (string, error) {
		secret := &corev1.Secret{}
		if err := g.k8sClinet.Get(ctx, client.ObjectKey{Name: secretName, Namespace: g.namespace}, secret); err != nil {
			return "", err
		}

		token := string(secret.Data[codebaseUtil.GitServerSecretTokenField])
		if token == "" {
			return "", errors.New("git provider token is empty")
		}

		return token, nil
	})
}
