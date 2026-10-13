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
)

const testNamespace = "krci"

// countingClient wraps a client.Client and counts Get calls so tests can assert that the
// settings cache avoids repeated Kubernetes reads.
type countingClient struct {
	client.Client
	gets int
}

func (c *countingClient) Get(
	ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption,
) error {
	c.gets++

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

func gitServerFixture() (*codebaseApi.GitServer, *corev1.Secret) {
	return newGitServer("gitlab", codebaseApi.GitProviderGitlab, "gitlab.example.com"), newTokenSecret("gitlab", "t0ken")
}

func TestGetGitProviderSettings_CachesAcrossCalls(t *testing.T) {
	gs, secret := gitServerFixture()
	svc, cc := newTestService(t, gs, secret)

	s1, err := svc.GetGitProviderSettings(context.Background(), "gitlab")
	require.NoError(t, err)
	assert.Equal(t, "gitlab", s1.GitProvider)
	assert.Equal(t, "t0ken", s1.Token)
	assert.Equal(t, 2, cc.gets, "cold call should read GitServer CR + Secret")

	s2, err := svc.GetGitProviderSettings(context.Background(), "gitlab")
	require.NoError(t, err)
	assert.Equal(t, s1, s2)
	assert.Equal(t, 2, cc.gets, "second call within TTL must be served from cache (no extra k8s reads)")
}

func TestGetGitProviderSettings_DoesNotCacheNotFound(t *testing.T) {
	// No GitServer CR exists -> NotFound. The error must not be cached, so the next call retries.
	svc, cc := newTestService(t)

	_, err := svc.GetGitProviderSettings(context.Background(), "missing")
	require.Error(t, err)

	first := cc.gets

	_, err = svc.GetGitProviderSettings(context.Background(), "missing")
	require.Error(t, err)
	assert.Greater(t, cc.gets, first, "NotFound must not be cached; the lookup should retry")
}

func TestGetGitProviderSettings_DoesNotCacheEmptyToken(t *testing.T) {
	gs, _ := gitServerFixture()
	svc, cc := newTestService(t, gs, newTokenSecret("gitlab", ""))

	_, err := svc.GetGitProviderSettings(context.Background(), "gitlab")
	require.Error(t, err)

	first := cc.gets

	_, err = svc.GetGitProviderSettings(context.Background(), "gitlab")
	require.Error(t, err)
	assert.Greater(t, cc.gets, first, "empty-token error must not be cached; the lookup should retry")
}

func TestGetGitProviderSettingsList_ReturnsSettingsPerGitServerInNamespace(t *testing.T) {
	otherNamespace := newGitServer("other", codebaseApi.GitProviderGitlab, "gitlab.other.example.com")
	otherNamespace.Namespace = "other"
	// The secret exists in testNamespace: without the namespace filter this GitServer resolves.
	otherNamespace.Spec.NameSshKeySecret = tokenSecretName("corp-gitlab")
	svc, _ := newTestService(t,
		newGitServer("corp-gitlab", codebaseApi.GitProviderGitlab, "gitlab.example.com"),
		newTokenSecret("corp-gitlab", "gl-t0ken"),
		newGitServer("corp-github", codebaseApi.GitProviderGithub, "github.com"),
		newTokenSecret("corp-github", "gh-t0ken"),
		otherNamespace,
	)

	settings, err := svc.GetGitProviderSettingsList(context.Background())

	require.NoError(t, err)
	assert.ElementsMatch(t, []GitServerSettings{
		{Url: "https://gitlab.example.com", Token: "gl-t0ken", GitProvider: "gitlab", GitServerName: "corp-gitlab"},
		{Url: "https://api.github.com", Token: "gh-t0ken", GitProvider: "github", GitServerName: "corp-github"},
	}, settings)
}

func TestGetGitProviderSettingsList_FailsWhenSecretIsMissing(t *testing.T) {
	gs, _ := gitServerFixture()
	svc, _ := newTestService(t, gs)

	settings, err := svc.GetGitProviderSettingsList(context.Background())

	require.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err), "expected the secret NotFound error, got: %v", err)
	assert.Contains(t, err.Error(), "failed to get settings for GitServer gitlab")
	assert.Nil(t, settings)
}

func TestGetGitProviderSettingsList_FailsWhenListFails(t *testing.T) {
	listErr := errors.New("api server unavailable")
	svc := NewGitServerService(failingListClient{err: listErr}, testNamespace)

	settings, err := svc.GetGitProviderSettingsList(context.Background())

	require.ErrorIs(t, err, listErr)
	assert.Contains(t, err.Error(), "failed to list GitServers")
	assert.Nil(t, settings)
}
