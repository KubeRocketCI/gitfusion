package organizations

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
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

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

const testNamespace = "krci"

// recordingProvider records the GitServers it is asked for and fails for those in failFor.
type recordingProvider struct {
	mu      sync.Mutex
	seen    []string
	failFor map[string]bool
}

func (p *recordingProvider) ListUserOrganizations(
	_ context.Context, settings krci.GitServerSettings,
) ([]models.Organization, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.seen = append(p.seen, settings.GitServerName)

	if p.failFor[settings.GitServerName] {
		return nil, errors.New("provider unavailable")
	}

	return []models.Organization{{Name: settings.GitServerName}}, nil
}

// secretRecorder records the names of the Secrets read through it.
type secretRecorder struct {
	client.Client

	mu      sync.Mutex
	secrets []string
}

func (c *secretRecorder) Get(
	ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption,
) error {
	if _, ok := obj.(*corev1.Secret); ok {
		c.mu.Lock()
		c.secrets = append(c.secrets, key.Name)
		c.mu.Unlock()
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

func newGitServer(name, provider string) *codebaseApi.GitServer {
	return &codebaseApi.GitServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: codebaseApi.GitServerSpec{
			GitProvider:      provider,
			GitHost:          name + ".example.com",
			NameSshKeySecret: name + "-secret",
		},
	}
}

func newSecret(gitServerName string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: gitServerName + "-secret", Namespace: testNamespace},
		Data:       data,
	}
}

func tokenData() map[string][]byte {
	return map[string][]byte{codebaseUtil.GitServerSecretTokenField: []byte("t0ken")}
}

func newWarmUpService(
	t *testing.T, provider OrganizationsProvider, objs ...client.Object,
) (*OrganizationsService, *secretRecorder) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, codebaseApi.AddToScheme(scheme))

	recorder := &secretRecorder{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()}

	multi := NewMultiProviderOrganizationsService()
	multi.providers["gitlab"] = provider
	multi.providers["github"] = provider

	return NewOrganizationsService(multi, krci.NewGitServerService(recorder, testNamespace)), recorder
}

// captureLogs routes info and error records of the default slog logger to the returned buffer for the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &buf
}

func TestOrganizationsService_WarmUp_SkipsUnsupportedLogsBrokenAndWarmsSupported(t *testing.T) {
	provider := &recordingProvider{}
	svc, recorder := newWarmUpService(t, provider,
		newGitServer("gerrit", codebaseApi.GitProviderGerrit),
		newSecret("gerrit", map[string][]byte{codebaseUtil.PrivateSShKeyName: []byte("private-key")}),
		newGitServer("broken", codebaseApi.GitProviderGithub),
		newGitServer("good", codebaseApi.GitProviderGitlab),
		newSecret("good", tokenData()),
	)
	logs := captureLogs(t)

	svc.WarmUp(context.Background())

	assert.Equal(t, []string{"good"}, provider.seen)
	assert.ElementsMatch(t, []string{"broken-secret", "good-secret"}, recorder.secrets, "gerrit Secret must not be read")
	assert.Contains(t, logs.String(), "level=ERROR")
	assert.Contains(t, logs.String(), "gitServerName=broken")
	assert.NotContains(t, logs.String(), "gerrit", "an unsupported GitServer is skipped without an info or error log")
	assert.Contains(t, logs.String(), "Warmed up organization cache")

	cached, err := svc.GetProvider().ListUserOrganizations(
		context.Background(), krci.GitServerSettings{GitProvider: "gitlab", GitServerName: "good"},
	)
	require.NoError(t, err)
	assert.Equal(t, []models.Organization{{Name: "good"}}, cached)
	assert.Equal(t, []string{"good"}, provider.seen, "the cache serves the second request")
}

func TestOrganizationsService_WarmUp_ProviderFailureDoesNotStopOtherServers(t *testing.T) {
	provider := &recordingProvider{failFor: map[string]bool{"bad": true}}
	svc, _ := newWarmUpService(t, provider,
		newGitServer("bad", codebaseApi.GitProviderGitlab), newSecret("bad", tokenData()),
		newGitServer("good", codebaseApi.GitProviderGithub), newSecret("good", tokenData()),
	)
	logs := captureLogs(t)

	svc.WarmUp(context.Background())

	assert.ElementsMatch(t, []string{"bad", "good"}, provider.seen)
	assert.Contains(t, logs.String(), "Failed to list user organizations")
	assert.Contains(t, logs.String(), "gitServerName=bad")
}
