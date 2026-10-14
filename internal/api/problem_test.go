package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	codebaseApi "github.com/epam/edp-codebase-operator/v2/api/v1"
	codebaseUtil "github.com/epam/edp-codebase-operator/v2/pkg/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viccon/sturdyc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/branches"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	"github.com/KubeRocketCI/gitfusion/internal/services/organizations"
	"github.com/KubeRocketCI/gitfusion/internal/services/repositories"
)

const (
	// leakMarker is embedded in causes; it must never reach a response body.
	leakMarker = "https://upstream.internal/secret-path"

	genericDetail  = "The request could not be completed. Contact your administrator with the request ID."
	notFoundDetail = "The requested resource was not found."
)

// requestLogLine is the subset of the httplog JSON line the tests read.
type requestLogLine struct {
	Error       string `json:"error"`
	HTTPRequest struct {
		RequestID string `json:"requestID"`
	} `json:"httpRequest"`
}

func requireInvalidParam(t *testing.T, err error, param string) {
	t.Helper()

	var invalid *gferrors.InvalidParamError

	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, param, invalid.Param)
}

func decodeRequestLogLine(t *testing.T, logs *bytes.Buffer) requestLogLine {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n"))
	require.Len(t, lines, 1, "expected exactly one log line, got: %s", logs.String())

	var line requestLogLine

	require.NoError(t, json.Unmarshal(lines[0], &line))

	return line
}

// newRequestLogRouter returns a router whose request log lines are written to logs as JSON.
func newRequestLogRouter(logs *bytes.Buffer) chi.Router {
	logger := httplog.NewLogger("gitfusion-test", httplog.Options{
		JSON:     true,
		Concise:  true,
		LogLevel: slog.LevelDebug,
		Writer:   logs,
	})

	router := chi.NewRouter()
	router.Use(httplog.RequestLogger(logger))

	return router
}

// requireProblemResponse asserts the problem+json contract of rec and returns the request log line.
// The caller asserts the logged cause; the helper asserts it is absent from the body.
func requireProblemResponse(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	logs *bytes.Buffer,
	wantStatus int,
	wantDetail string,
) requestLogLine {
	t.Helper()

	titles := map[int]string{
		http.StatusBadRequest:          "Bad Request",
		http.StatusNotFound:            "Not Found",
		http.StatusBadGateway:          "Bad Gateway",
		http.StatusInternalServerError: "Internal Server Error",
	}

	res := rec.Result()

	assert.Equal(t, wantStatus, res.StatusCode)
	assert.Equal(t, "application/problem+json", res.Header.Get("Content-Type"))

	var problem models.Problem

	decoder := json.NewDecoder(bytes.NewReader(rec.Body.Bytes()))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&problem))

	assert.Equal(t, "about:blank", problem.Type)
	assert.Equal(t, titles[wantStatus], problem.Title)
	assert.Equal(t, wantStatus, problem.Status)
	assert.Equal(t, wantDetail, problem.Detail)

	requestID := res.Header.Get("X-Request-Id")
	require.NotEmpty(t, requestID)
	require.NotNil(t, problem.RequestId)
	assert.Equal(t, requestID, *problem.RequestId)

	line := decodeRequestLogLine(t, logs)
	assert.Equal(t, requestID, line.HTTPRequest.RequestID)

	require.NotEmpty(t, line.Error)
	assert.NotContains(t, rec.Body.String(), leakMarker)
	assert.NotContains(t, rec.Body.String(), line.Error)

	return line
}

func TestNewHTTPHandlerErrorResponses(t *testing.T) {
	const (
		pullRequestsTarget = "/api/v1/pull-requests?gitServer=gs&owner=o&repoName=r"
		pipelinesTarget    = "/api/v1/pipelines?gitServer=gs&project=p"
		jobsTarget         = "/api/v1/pipeline-jobs?gitServer=gs&project=p&pipelineId="
	)

	escapedMarker := url.QueryEscape(leakMarker)

	tests := []struct {
		name       string
		target     string
		serviceErr error
		wantStatus int
		wantDetail string
		// wantCause is the expected log field for errors raised before a service is called.
		wantCause string
	}{
		{
			name:       "missing required parameter",
			target:     "/api/v1/pipelines?project=p",
			wantStatus: http.StatusBadRequest,
			wantDetail: "Missing required parameter 'gitServer'.",
			wantCause:  "Query argument gitServer is required, but not found",
		},
		{
			name:       "non-integer page",
			target:     pullRequestsTarget + "&page=" + escapedMarker,
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'page'.",
			wantCause: "Invalid format for parameter page: error binding string parameter: " +
				fmt.Sprintf("strconv.ParseInt: parsing %q: invalid syntax", leakMarker),
		},
		{
			name:       "repeated page",
			target:     pullRequestsTarget + "&page=1&page=2",
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'page'.",
			wantCause:  "Invalid format for parameter page: multiple values for single value parameter 'page'",
		},
		{
			name:       "non-numeric pipelineId",
			target:     jobsTarget + escapedMarker,
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'pipelineId'.",
			wantCause: "invalid parameter pipelineId: " +
				fmt.Sprintf("strconv.Atoi: parsing %q: invalid syntax", leakMarker),
		},
		{
			name:   "invalid parameter error from a service",
			target: pullRequestsTarget,
			serviceErr: fmt.Errorf("list: %w", &gferrors.InvalidParamError{
				Param: "project",
				Err:   errors.New("bad project " + leakMarker),
			}),
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'project'.",
		},
		{
			name:   "unsupported operation",
			target: jobsTarget + "7",
			serviceErr: fmt.Errorf("provider github does not support pipeline jobs %s: %w",
				leakMarker, gferrors.ErrUnsupportedOperation),
			wantStatus: http.StatusBadRequest,
			wantDetail: "The operation is not supported for this git server.",
		},
		{
			name:       "not found",
			target:     pullRequestsTarget,
			serviceErr: fmt.Errorf("repository %s: %w", leakMarker, gferrors.ErrNotFound),
			wantStatus: http.StatusNotFound,
			wantDetail: notFoundDetail,
		},
		{
			name:       "unauthorized",
			target:     pipelinesTarget,
			serviceErr: fmt.Errorf("GET %s: 401: %w", leakMarker, gferrors.ErrUnauthorized),
			wantStatus: http.StatusBadGateway,
			wantDetail: genericDetail,
		},
		{
			name:       "unclassified error",
			target:     pipelinesTarget,
			serviceErr: errors.New("dial tcp " + leakMarker + ": connection refused"),
			wantStatus: http.StatusInternalServerError,
			wantDetail: genericDetail,
		},
		{
			name:   "joined not found",
			target: pullRequestsTarget,
			serviceErr: errors.Join(
				fmt.Errorf("refresh %s: %w", leakMarker, gferrors.ErrNotFound),
				sturdyc.ErrOnlyCachedRecords,
			),
			wantStatus: http.StatusNotFound,
			wantDetail: notFoundDetail,
		},
		{
			name:   "joined unauthorized",
			target: pipelinesTarget,
			serviceErr: errors.Join(
				fmt.Errorf("refresh %s: %w", leakMarker, gferrors.ErrUnauthorized),
				sturdyc.ErrOnlyCachedRecords,
			),
			wantStatus: http.StatusBadGateway,
			wantDetail: genericDetail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer

			pipelines := &stubPipelineService{
				triggerErr: tt.serviceErr,
				listErr:    tt.serviceErr,
				jobsErr:    tt.serviceErr,
				traceErr:   tt.serviceErr,
			}
			pullRequests := &stubPullRequestLister{err: tt.serviceErr}

			handler := NewHTTPHandler(newStrictHandler(NewServer(
				nil, nil, nil, nil,
				NewPipelineHandler(pipelines),
				NewPullRequestHandler(pullRequests),
			)), newRequestLogRouter(&logs))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))

			wantCause := tt.wantCause
			if tt.serviceErr != nil {
				wantCause = tt.serviceErr.Error()
			}

			line := requireProblemResponse(t, rec, &logs, tt.wantStatus, tt.wantDetail)
			assert.Equal(t, wantCause, line.Error)
		})
	}
}

func TestNewHTTPHandlerOmitsRequestIDWhenAbsent(t *testing.T) {
	handler := NewHTTPHandler(newStrictHandler(NewServer(
		nil, nil, nil, nil,
		NewPipelineHandler(&stubPipelineService{listErr: gferrors.ErrNotFound}),
		NewPullRequestHandler(&stubPullRequestLister{}),
	)), chi.NewRouter())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/pipelines?gitServer=gs&project=p", nil))

	res := rec.Result()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	assert.Equal(t, "application/problem+json", res.Header.Get("Content-Type"))
	assert.Empty(t, res.Header.Values("X-Request-Id"))

	var problem models.Problem

	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
	assert.Nil(t, problem.RequestId)
	assert.NotContains(t, rec.Body.String(), "requestId")
}

const (
	fixtureNamespace  = "krci"
	gerritServerName  = "gerrit-server"
	brokenServerName  = "broken-server"
	missingServerName = "missing-server"
)

// newServiceBackedServer wires the repository, branch, organization and cache handlers to real services
// over a fake Kubernetes client. The client holds two GitServers whose provider has no implementation:
// gerrit-server with its Secret, and broken-server whose Secret is absent. The organization warm-up
// goroutine lists them and fails before any provider call, so no request leaves the process.
func newServiceBackedServer(t *testing.T) *Server {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, codebaseApi.AddToScheme(scheme))

	newGitServer := func(name string) *codebaseApi.GitServer {
		return &codebaseApi.GitServer{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: fixtureNamespace},
			Spec: codebaseApi.GitServerSpec{
				GitProvider:      codebaseApi.GitProviderGerrit,
				GitHost:          name + ".example.com",
				NameSshKeySecret: name + "-secret",
			},
		}
	}
	gerrit := newGitServer(gerritServerName)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: gerrit.Spec.NameSshKeySecret, Namespace: fixtureNamespace},
		Data:       map[string][]byte{codebaseUtil.GitServerSecretTokenField: []byte("t0ken")},
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(gerrit, secret, newGitServer(brokenServerName)).Build()
	gitServerService := krci.NewGitServerService(k8sClient, fixtureNamespace)

	repoSvc := repositories.NewRepositoriesService(repositories.NewMultiProviderRepositoryService(), gitServerService)
	orgSvc := organizations.NewOrganizationsService(
		organizations.NewMultiProviderOrganizationsService(gitServerService), gitServerService,
	)
	branchesSvc := branches.NewBranchesService(branches.NewMultiProviderBranchesService(), gitServerService)

	cacheManager := cache.NewManager(
		repoSvc.GetProvider().GetCache(),
		orgSvc.GetProvider().GetCache(),
		branchesSvc.GetProvider().GetCache(),
		cache.NewPullRequestCache(),
		cache.NewPipelineCache(),
		cache.NewPipelineJobsCache(),
		cache.NewPipelineJobTraceCache(),
	)

	return NewServer(
		NewRepositoryHandler(repoSvc),
		NewOrganizationHandler(orgSvc),
		NewBranchHandler(branchesSvc),
		NewCacheHandler(cacheManager),
		nil, nil,
	)
}

func TestNewHTTPHandlerServiceBackedErrorResponses(t *testing.T) {
	const (
		unsupportedDetail  = "The operation is not supported for this git server."
		unsupportedCause   = "unsupported provider gerrit"
		missingCause       = `"missing-server" not found`
		missingSecretCause = `secrets "` + brokenServerName + `-secret" not found`

		repositoriesTarget = "/api/v1/repositories?owner=o&gitServer="
		repositoryTarget   = "/api/v1/repository?owner=o&repoName=r&gitServer="
		branchesTarget     = "/api/v1/branches?owner=o&repoName=r&gitServer="
		organizationTarget = "/api/v1/user/organizations?gitServer="
	)

	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantDetail string
		// wantCause is a substring of the logged error.
		wantCause string
	}{
		{
			name:       "repositories unsupported provider",
			method:     http.MethodGet,
			target:     repositoriesTarget + gerritServerName,
			wantStatus: http.StatusBadRequest,
			wantDetail: unsupportedDetail,
			wantCause:  unsupportedCause,
		},
		{
			name:       "repositories missing git server",
			method:     http.MethodGet,
			target:     repositoriesTarget + missingServerName,
			wantStatus: http.StatusNotFound,
			wantDetail: notFoundDetail,
			wantCause:  missingCause,
		},
		{
			name:       "repository unsupported provider",
			method:     http.MethodGet,
			target:     repositoryTarget + gerritServerName,
			wantStatus: http.StatusBadRequest,
			wantDetail: unsupportedDetail,
			wantCause:  unsupportedCause,
		},
		{
			name:       "repository missing git server",
			method:     http.MethodGet,
			target:     repositoryTarget + missingServerName,
			wantStatus: http.StatusNotFound,
			wantDetail: notFoundDetail,
			wantCause:  missingCause,
		},
		{
			name:       "branches unsupported provider",
			method:     http.MethodGet,
			target:     branchesTarget + gerritServerName,
			wantStatus: http.StatusBadRequest,
			wantDetail: unsupportedDetail,
			wantCause:  unsupportedCause,
		},
		{
			name:       "branches missing git server",
			method:     http.MethodGet,
			target:     branchesTarget + missingServerName,
			wantStatus: http.StatusNotFound,
			wantDetail: notFoundDetail,
			wantCause:  missingCause,
		},
		{
			name:       "organizations unsupported provider",
			method:     http.MethodGet,
			target:     organizationTarget + gerritServerName,
			wantStatus: http.StatusBadRequest,
			wantDetail: unsupportedDetail,
			wantCause:  unsupportedCause,
		},
		{
			name:       "organizations missing git server",
			method:     http.MethodGet,
			target:     organizationTarget + missingServerName,
			wantStatus: http.StatusNotFound,
			wantDetail: notFoundDetail,
			wantCause:  missingCause,
		},
		{
			name:       "repositories git server with missing secret",
			method:     http.MethodGet,
			target:     repositoriesTarget + brokenServerName,
			wantStatus: http.StatusInternalServerError,
			wantDetail: genericDetail,
			wantCause:  missingSecretCause,
		},
		{
			name:       "cache invalidate unknown endpoint",
			method:     http.MethodDelete,
			target:     "/api/v1/cache/invalidate?endpoint=bogus",
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'endpoint'.",
			wantCause:  "unsupported endpoint bogus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer

			handler := NewHTTPHandler(newStrictHandler(newServiceBackedServer(t)), newRequestLogRouter(&logs))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, nil))

			line := requireProblemResponse(t, rec, &logs, tt.wantStatus, tt.wantDetail)
			assert.Contains(t, line.Error, tt.wantCause)
		})
	}
}
