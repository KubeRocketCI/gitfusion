package krci

import (
	"context"
	"errors"
	"testing"

	codebaseApi "github.com/epam/edp-codebase-operator/v2/api/v1"
	codebaseUtil "github.com/epam/edp-codebase-operator/v2/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
)

const testNamespace = "krci"

// countingClient wraps a client.Client and counts Get calls so tests can assert which Kubernetes
// reads a call makes.
type countingClient struct {
	client.Client
	gets       int
	secretGets int
}

func (c *countingClient) Get(
	ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption,
) error {
	c.gets++

	if _, ok := obj.(*corev1.Secret); ok {
		c.secretGets++
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

func newTestService(t *testing.T, objs ...client.Object) (*GitServerService, *countingClient) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, codebaseApi.AddToScheme(scheme))

	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	cc := &countingClient{Client: base}

	return NewGitServerService(cc, testNamespace), cc
}

type failingListClient struct {
	client.Client
	err error
}

func (c failingListClient) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return c.err
}

type failingGetClient struct {
	client.Client
	err error
}

func (c failingGetClient) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return c.err
}

func tokenSecretName(gitServerName string) string {
	return gitServerName + "-secret"
}

// newGitServer returns a GitServer in testNamespace that references tokenSecretName(name).
func newGitServer(name, provider, host string) *codebaseApi.GitServer {
	return &codebaseApi.GitServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: codebaseApi.GitServerSpec{
			GitProvider:      provider,
			GitHost:          host,
			NameSshKeySecret: tokenSecretName(name),
		},
	}
}

func newTokenSecret(gitServerName, token string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tokenSecretName(gitServerName), Namespace: testNamespace},
		Data:       map[string][]byte{codebaseUtil.GitServerSecretTokenField: []byte(token)},
	}
}

// newSSHKeySecret returns a Gerrit-style GitServer Secret: an SSH key pair and a user name, no token.
func newSSHKeySecret(gitServerName string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tokenSecretName(gitServerName), Namespace: testNamespace},
		Data: map[string][]byte{
			codebaseUtil.PrivateSShKeyName: []byte("private-key"),
			"id_rsa.pub":                   []byte("public-key"),
			"username":                     []byte("ci-user"),
		},
	}
}

func gitServerFixture() (*codebaseApi.GitServer, *corev1.Secret) {
	return newGitServer("gitlab", codebaseApi.GitProviderGitlab, "gitlab.example.com"), newTokenSecret("gitlab", "t0ken")
}

func allowAll(string) error { return nil }

func TestResolve_CachesAcrossCalls(t *testing.T) {
	gs, secret := gitServerFixture()
	svc, cc := newTestService(t, gs, secret)

	s1, err := svc.Resolve(context.Background(), "gitlab", allowAll)
	require.NoError(t, err)
	assert.Equal(t, GitServerSettings{
		Url: "https://gitlab.example.com", Token: "t0ken", GitProvider: "gitlab", GitServerName: "gitlab",
	}, s1)
	assert.Equal(t, 2, cc.gets, "cold call should read GitServer CR + Secret")

	s2, err := svc.Resolve(context.Background(), "gitlab", allowAll)
	require.NoError(t, err)
	assert.Equal(t, s1, s2)
	assert.Equal(t, 2, cc.gets, "second call within TTL must be served from cache (no extra k8s reads)")
}

func TestResolve_PassesGitProviderToCheck(t *testing.T) {
	gs, secret := gitServerFixture()
	svc, _ := newTestService(t, gs, secret)

	var got string

	_, err := svc.Resolve(context.Background(), "gitlab", func(gitProvider string) error {
		got = gitProvider

		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, codebaseApi.GitProviderGitlab, got)
}

func TestResolve_UnsupportedCheckReadsNoSecret(t *testing.T) {
	svc, cc := newTestService(t,
		newGitServer("gerrit", codebaseApi.GitProviderGerrit, "gerrit.example.com"),
		newSSHKeySecret("gerrit"),
	)
	unsupported := gferrors.UnsupportedOperation("gerrit", "listRepositories")

	_, err := svc.Resolve(context.Background(), "gerrit", func(string) error { return unsupported })

	require.Same(t, unsupported, err)
	assert.Equal(t, 1, cc.gets, "unsupported call should read the GitServer CR only")
	assert.Zero(t, cc.secretGets)

	_, err = svc.Resolve(context.Background(), "gerrit", func(string) error { return unsupported })

	require.Same(t, unsupported, err)
	assert.Equal(t, 1, cc.gets, "the GitServer CR is served from cache on the second call")
	assert.Zero(t, cc.secretGets)
}

func TestResolve_MissingGitServerIsErrNotFound(t *testing.T) {
	svc, cc := newTestService(t)
	checked := false

	_, err := svc.Resolve(context.Background(), "missing", func(string) error {
		checked = true

		return nil
	})

	require.ErrorIs(t, err, gferrors.ErrNotFound)
	assert.True(t, apierrors.IsNotFound(err), "expected the GitServer NotFound error to stay in the chain, got: %v", err)
	assert.Contains(t, err.Error(), `git server missing`)
	assert.False(t, checked, "check must not run without a GitServer")
	assert.Zero(t, cc.secretGets)
}

func TestResolve_MissingSecretIsNotErrNotFound(t *testing.T) {
	gs, _ := gitServerFixture()
	svc, _ := newTestService(t, gs)

	_, err := svc.Resolve(context.Background(), "gitlab", allowAll)

	require.Error(t, err)
	assert.NotErrorIs(t, err, gferrors.ErrNotFound)
	assert.True(t, apierrors.IsNotFound(err), "expected the secret NotFound error, got: %v", err)
}

func TestResolve_GitServerLookupFailureIsNotErrNotFound(t *testing.T) {
	getErr := errors.New("api server unavailable")
	svc := NewGitServerService(failingGetClient{err: getErr}, testNamespace)

	_, err := svc.Resolve(context.Background(), "gitlab", allowAll)

	require.ErrorIs(t, err, getErr)
	assert.NotErrorIs(t, err, gferrors.ErrNotFound)
}

func TestResolve_DoesNotCacheNotFound(t *testing.T) {
	svc, cc := newTestService(t)

	_, err := svc.Resolve(context.Background(), "missing", allowAll)
	require.Error(t, err)

	first := cc.gets

	_, err = svc.Resolve(context.Background(), "missing", allowAll)
	require.Error(t, err)
	assert.Greater(t, cc.gets, first, "NotFound must not be cached; the lookup should retry")
}

func TestResolve_DoesNotCacheEmptyToken(t *testing.T) {
	gs, _ := gitServerFixture()
	svc, cc := newTestService(t, gs, newTokenSecret("gitlab", ""))

	_, err := svc.Resolve(context.Background(), "gitlab", allowAll)
	require.EqualError(t, err, "git provider token is empty")

	first := cc.secretGets

	_, err = svc.Resolve(context.Background(), "gitlab", allowAll)
	require.Error(t, err)
	assert.Greater(t, cc.secretGets, first, "empty-token error must not be cached; the Secret should be read again")
}

func TestResolve_SecretWithoutTokenFailsForEveryProvider(t *testing.T) {
	for _, provider := range []string{
		codebaseApi.GitProviderGithub,
		codebaseApi.GitProviderGitlab,
		codebaseApi.GitProviderBitbucket,
		codebaseApi.GitProviderGerrit,
	} {
		t.Run(provider, func(t *testing.T) {
			svc, _ := newTestService(t,
				newGitServer("gs", provider, provider+".example.com"),
				newSSHKeySecret("gs"),
			)

			_, err := svc.Resolve(context.Background(), "gs", allowAll)

			require.EqualError(t, err, "git provider token is empty")
		})
	}
}

func TestGitServerNames_ListsGitServersInNamespaceWithoutReadingSecrets(t *testing.T) {
	otherNamespace := newGitServer("other", codebaseApi.GitProviderGitlab, "gitlab.other.example.com")
	otherNamespace.Namespace = "other"
	svc, cc := newTestService(t,
		newGitServer("corp-gitlab", codebaseApi.GitProviderGitlab, "gitlab.example.com"),
		newTokenSecret("corp-gitlab", "gl-t0ken"),
		newGitServer("corp-gerrit", codebaseApi.GitProviderGerrit, "gerrit.example.com"),
		newSSHKeySecret("corp-gerrit"),
		otherNamespace,
	)

	names, err := svc.GitServerNames(context.Background())

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"corp-gitlab", "corp-gerrit"}, names)
	assert.Zero(t, cc.gets)
}

func TestGitServerNames_FailsWhenListFails(t *testing.T) {
	listErr := errors.New("api server unavailable")
	svc := NewGitServerService(failingListClient{err: listErr}, testNamespace)

	names, err := svc.GitServerNames(context.Background())

	require.ErrorIs(t, err, listErr)
	assert.Contains(t, err.Error(), "failed to list GitServers")
	assert.Nil(t, names)
}

func TestResolve_BuildsSettingsFromCurrentGitServer(t *testing.T) {
	gs, secret := gitServerFixture()
	svc, cc := newTestService(t, gs, secret)

	_, err := svc.Resolve(context.Background(), "gitlab", allowAll)
	require.NoError(t, err)

	changed := &codebaseApi.GitServer{}
	require.NoError(t, cc.Get(context.Background(), client.ObjectKeyFromObject(gs), changed))
	changed.Spec.GitProvider = codebaseApi.GitProviderGithub
	changed.Spec.GitHost = "github.com"
	require.NoError(t, cc.Update(context.Background(), changed))
	svc.gitServerCache.Delete("gitlab")

	var checked string

	settings, err := svc.Resolve(context.Background(), "gitlab", func(gitProvider string) error {
		checked = gitProvider

		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, codebaseApi.GitProviderGithub, checked)
	assert.Equal(t, GitServerSettings{
		Url: "https://api.github.com", Token: "t0ken", GitProvider: "github", GitServerName: "gitlab",
	}, settings, "settings must match the GitServer the check approved; only the token is cached")
	assert.Equal(t, 1, cc.secretGets, "the token stays cached for the unchanged Secret")
}

func TestGitServerNames_CachesListedGitServers(t *testing.T) {
	gs, secret := gitServerFixture()
	svc, cc := newTestService(t, gs, secret)

	_, err := svc.GitServerNames(context.Background())
	require.NoError(t, err)

	_, err = svc.Resolve(context.Background(), "gitlab", allowAll)

	require.NoError(t, err)
	assert.Equal(t, 1, cc.gets, "Resolve after GitServerNames should read the Secret only")
	assert.Equal(t, 1, cc.secretGets)
}
