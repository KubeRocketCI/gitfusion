package krci

import (
	"context"
	"testing"

	codebaseApi "github.com/epam/edp-codebase-operator/v2/api/v1"
	codebaseUtil "github.com/epam/edp-codebase-operator/v2/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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

	return NewGitServerService(cc, "krci"), cc
}

func gitServerFixture() (*codebaseApi.GitServer, *corev1.Secret) {
	gs := &codebaseApi.GitServer{
		ObjectMeta: metav1.ObjectMeta{Name: "gitlab", Namespace: "krci"},
		Spec: codebaseApi.GitServerSpec{
			GitProvider:      codebaseApi.GitProviderGitlab,
			GitHost:          "gitlab.example.com",
			NameSshKeySecret: "gitlab-secret",
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "gitlab-secret", Namespace: "krci"},
		Data:       map[string][]byte{codebaseUtil.GitServerSecretTokenField: []byte("t0ken")},
	}

	return gs, secret
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
	emptySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "gitlab-secret", Namespace: "krci"},
		Data:       map[string][]byte{codebaseUtil.GitServerSecretTokenField: []byte("")},
	}
	svc, cc := newTestService(t, gs, emptySecret)

	_, err := svc.GetGitProviderSettings(context.Background(), "gitlab")
	require.Error(t, err)

	first := cc.gets

	_, err = svc.GetGitProviderSettings(context.Background(), "gitlab")
	require.Error(t, err)
	assert.Greater(t, cc.gets, first, "empty-token error must not be cached; the lookup should retry")
}
