package api

import (
	"bytes"
	"context"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
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
			name:       "empty required parameter",
			target:     "/api/v1/pipelines?gitServer=gs&project=",
			wantStatus: http.StatusBadRequest,
			wantDetail: "Missing required parameter 'project'.",
			wantCause:  "Query argument project is required, but not found",
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
			name:       "unknown pull request state",
			target:     pullRequestsTarget + "&state=" + escapedMarker,
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'state'.",
			wantCause:  fmt.Sprintf("invalid parameter state: unsupported value %q", leakMarker),
		},
		{
			name:       "upper-case pull request state",
			target:     pullRequestsTarget + "&state=OPEN",
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'state'.",
			wantCause:  `invalid parameter state: unsupported value "OPEN"`,
		},
		{
			name:       "empty pull request state",
			target:     pullRequestsTarget + "&state=",
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'state'.",
			wantCause:  `invalid parameter state: unsupported value ""`,
		},
		{
			name:       "unknown pipeline status",
			target:     pipelinesTarget + "&status=" + escapedMarker,
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'status'.",
			wantCause:  fmt.Sprintf("invalid parameter status: unsupported value %q", leakMarker),
		},
		{
			name:       "provider spelling of a pipeline status",
			target:     pipelinesTarget + "&status=canceled",
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'status'.",
			wantCause:  `invalid parameter status: unsupported value "canceled"`,
		},
		{
			name:       "empty pipeline status",
			target:     pipelinesTarget + "&status=",
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'status'.",
			wantCause:  `invalid parameter status: unsupported value ""`,
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

			if tt.serviceErr == nil {
				assert.Empty(t, pullRequests.gotGitServer, "pull request service must not be called")
				assert.Empty(t, pipelines.gotListGitServer, "pipeline service must not be called")
			}
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
	fixtureNamespace         = "krci"
	gerritServerName         = "gerrit-server"
	gerritNoSecretServerName = "gerrit-nosecret-server"
	githubServerName         = "github-server"
	brokenServerName         = "broken-server"
	missingServerName        = "missing-server"
	tokenlessServerName      = "tokenless-server"
)

// getCounter counts Gets by object type.
type getCounter struct {
	client.Client

	secrets    int
	gitServers int
}

func (c *getCounter) Get(
	ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption,
) error {
	switch obj.(type) {
	case *corev1.Secret:
		c.secrets++
	case *codebaseApi.GitServer:
		c.gitServers++
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

// newServiceBackedServer returns buildServer over a fake Kubernetes client that counts Gets. The client holds:
//   - gerrit-server: Gerrit with an SSH-key Secret and no token;
//   - gerrit-nosecret-server: Gerrit whose Secret is absent;
//   - github-server: GitHub with a token;
//   - broken-server: GitHub whose Secret is absent;
//   - tokenless-server: GitHub whose Secret has no token.
//
// Every request routed to these servers fails before a provider call, so no request leaves the process.
func newServiceBackedServer(t *testing.T) (*Server, *getCounter) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, codebaseApi.AddToScheme(scheme))

	newGitServer := func(name, provider string) *codebaseApi.GitServer {
		return &codebaseApi.GitServer{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: fixtureNamespace},
			Spec: codebaseApi.GitServerSpec{
				GitProvider:      provider,
				GitHost:          name + ".example.com",
				NameSshKeySecret: name + "-secret",
			},
		}
	}
	newSecret := func(gitServerName string, data map[string][]byte) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: gitServerName + "-secret", Namespace: fixtureNamespace},
			Data:       data,
		}
	}
	sshKeyData := map[string][]byte{
		codebaseUtil.PrivateSShKeyName: []byte("private-key"),
		"id_rsa.pub":                   []byte("public-key"),
		"username":                     []byte("ci-user"),
	}

	counter := &getCounter{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		newGitServer(gerritServerName, codebaseApi.GitProviderGerrit), newSecret(gerritServerName, sshKeyData),
		newGitServer(gerritNoSecretServerName, codebaseApi.GitProviderGerrit),
		newGitServer(githubServerName, codebaseApi.GitProviderGithub),
		newSecret(githubServerName, map[string][]byte{codebaseUtil.GitServerSecretTokenField: []byte("t0ken")}),
		newGitServer(brokenServerName, codebaseApi.GitProviderGithub),
		newGitServer(tokenlessServerName, codebaseApi.GitProviderGithub), newSecret(tokenlessServerName, sshKeyData),
	).Build()}

	server, _ := buildServer(counter, fixtureNamespace)

	return server, counter
}

func TestNewHTTPHandlerServiceBackedErrorResponses(t *testing.T) {
	const (
		unsupportedDetail  = "The operation is not supported for this git server."
		missingCause       = `"missing-server" not found`
		missingSecretCause = `secrets "` + brokenServerName + `-secret" not found`

		repositoriesTarget    = "/api/v1/repositories?owner=o&gitServer="
		repositoryTarget      = "/api/v1/repository?owner=o&repoName=r&gitServer="
		branchesTarget        = "/api/v1/branches?owner=o&repoName=r&gitServer="
		organizationTarget    = "/api/v1/user/organizations?gitServer="
		pullRequestsTarget    = "/api/v1/pull-requests?owner=o&repoName=r&gitServer="
		pipelinesTarget       = "/api/v1/pipelines?project=p&gitServer="
		triggerPipelineTarget = "/api/v1/trigger-pipeline?project=p&ref=main&gitServer="
		pipelineJobsTarget    = "/api/v1/pipeline-jobs?project=p&pipelineId=1&gitServer="
		jobTraceTarget        = "/api/v1/pipeline-job-trace?project=p&jobId=1&gitServer="
	)

	type testCase struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantDetail string
		// wantCause is a substring of the logged error.
		wantCause string
		// wantSecretGets is the number of Secret reads the request makes.
		wantSecretGets int
	}

	type route struct {
		name      string
		method    string
		target    string
		operation string
	}

	routes := []route{
		{"repositories", http.MethodGet, repositoriesTarget, "listRepositories"},
		{"repository", http.MethodGet, repositoryTarget, "getRepository"},
		{"branches", http.MethodGet, branchesTarget, "listBranches"},
		{"organizations", http.MethodGet, organizationTarget, "listUserOrganizations"},
		{"pull requests", http.MethodGet, pullRequestsTarget, "listPullRequests"},
		{"pipelines", http.MethodGet, pipelinesTarget, "listPipelines"},
		{"trigger pipeline", http.MethodPost, triggerPipelineTarget, "triggerPipeline"},
		{"pipeline jobs", http.MethodGet, pipelineJobsTarget, "listPipelineJobs"},
		{"pipeline job trace", http.MethodGet, jobTraceTarget, "getPipelineJobTrace"},
	}

	unsupported := func(name, gitServer, gitProvider string, r route) testCase {
		return testCase{
			name: r.name + " " + name, method: r.method, target: r.target + gitServer,
			wantStatus: http.StatusBadRequest, wantDetail: unsupportedDetail,
			wantCause: "provider " + gitProvider + " does not support " + r.operation,
		}
	}

	tests := make([]testCase, 0, len(routes)*2)

	for _, r := range routes {
		tests = append(tests, unsupported("gerrit with an SSH-key secret", gerritServerName, "gerrit", r))
		tests = append(tests, unsupported("gerrit without a secret", gerritNoSecretServerName, "gerrit", r))
	}

	// GitHub implements everything except these three.
	for _, r := range routes {
		switch r.operation {
		case "triggerPipeline", "listPipelineJobs", "getPipelineJobTrace":
			tests = append(tests, unsupported("github unsupported operation", githubServerName, "github", r))
		}
	}

	for _, r := range routes[:4] {
		tests = append(tests, testCase{
			name: r.name + " missing git server", method: r.method, target: r.target + missingServerName,
			wantStatus: http.StatusNotFound, wantDetail: notFoundDetail, wantCause: missingCause,
		})
	}

	tests = append(tests,
		testCase{
			name:       "repositories git server with missing secret",
			method:     http.MethodGet,
			target:     repositoriesTarget + brokenServerName,
			wantStatus: http.StatusInternalServerError,
			wantDetail: genericDetail,
			wantCause:  missingSecretCause,

			wantSecretGets: 1,
		},
		testCase{
			name:       "pull requests token provider without a token",
			method:     http.MethodGet,
			target:     pullRequestsTarget + tokenlessServerName,
			wantStatus: http.StatusInternalServerError,
			wantDetail: genericDetail,
			wantCause:  "git provider token is empty",

			wantSecretGets: 1,
		},
		testCase{
			name:       "cache invalidate unknown endpoint",
			method:     http.MethodDelete,
			target:     "/api/v1/cache/invalidate?endpoint=bogus",
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for parameter 'endpoint'.",
			wantCause:  "unsupported endpoint bogus",
		},
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer

			server, counter := newServiceBackedServer(t)
			handler := NewHTTPHandler(newStrictHandler(server), newRequestLogRouter(&logs))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, nil))

			line := requireProblemResponse(t, rec, &logs, tt.wantStatus, tt.wantDetail)
			assert.Contains(t, line.Error, tt.wantCause)
			assert.Equal(t, tt.wantSecretGets, counter.secrets)
		})
	}
}
