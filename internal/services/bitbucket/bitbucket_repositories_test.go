package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	"github.com/KubeRocketCI/gitfusion/pkg/pointer"
)

func writeBitbucketJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	assert.NoError(t, json.NewEncoder(w).Encode(body))
}

// bitbucketErrorBody is the compact JSON body of bitbucketErrorHandler; the handler adds a trailing newline.
func bitbucketErrorBody(status int) string {
	return fmt.Sprintf(`{"type":"error","error":{"message":%q}}`, http.StatusText(status))
}

func bitbucketErrorHandler(t *testing.T, status int) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, _ *http.Request) {
		writeBitbucketJSON(t, w, status, json.RawMessage(bitbucketErrorBody(status)))
	}
}

type bitbucketErrorCase struct {
	name       string
	status     int
	wantErrIs  error
	wantErr    string
	wantDetail string
}

// bitbucketErrorCases covers the statuses mapped to domain sentinels and one passed through as a generic error.
// wantErr of a sentinel row is the subject prefix; detail returns the upstream text that follows the sentinel.
func bitbucketErrorCases(
	notFoundMsg, unauthorizedMsg, genericMsg string, detail func(status int) string,
) []bitbucketErrorCase {
	return []bitbucketErrorCase{
		{
			name: "not found", status: http.StatusNotFound,
			wantErrIs: gferrors.ErrNotFound, wantErr: notFoundMsg, wantDetail: detail(http.StatusNotFound),
		},
		{
			name: "unauthorized", status: http.StatusUnauthorized,
			wantErrIs: gferrors.ErrUnauthorized, wantErr: unauthorizedMsg, wantDetail: detail(http.StatusUnauthorized),
		},
		{
			name: "forbidden", status: http.StatusForbidden,
			wantErrIs: gferrors.ErrUnauthorized, wantErr: unauthorizedMsg, wantDetail: detail(http.StatusForbidden),
		},
		{name: "other API error", status: http.StatusBadRequest, wantErr: genericMsg},
	}
}

// bitbucketStatusDetail is the status line and body go-bitbucket puts in its error for bitbucketErrorHandler responses.
func bitbucketStatusDetail(status int) string {
	return fmt.Sprintf("%d %s, body: %s", status, http.StatusText(status), bitbucketErrorBody(status))
}

// bitbucketRESTDetail is the status and body the REST calls put in their errors for bitbucketErrorHandler responses.
func bitbucketRESTDetail(status int) string {
	return fmt.Sprintf("status %d, body: %s", status, bitbucketErrorBody(status))
}

// assertBitbucketError checks that only the sentinel statuses map to a domain error.
// A sentinel error reads "<subject>: <sentinel>: <upstream detail>".
func assertBitbucketError(t *testing.T, err error, tc bitbucketErrorCase) {
	t.Helper()

	require.Error(t, err)

	if tc.wantErrIs != nil {
		assert.ErrorIs(t, err, tc.wantErrIs)
		assert.Equal(t, tc.wantErr+": "+tc.wantErrIs.Error()+": "+tc.wantDetail, err.Error())

		return
	}

	assert.Contains(t, err.Error(), tc.wantErr)
	assert.NotErrorIs(t, err, gferrors.ErrNotFound)
	assert.NotErrorIs(t, err, gferrors.ErrUnauthorized)
}

// newBitbucketTestService returns the production service with every request redirected to mux.
// It clears BITBUCKET_API_BASE_URL so go-bitbucket uses the Bitbucket Cloud paths mux serves.
func newBitbucketTestService(t *testing.T, mux *http.ServeMux) *BitbucketService {
	t.Helper()
	t.Setenv("BITBUCKET_API_BASE_URL", "")

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	svc := NewBitbucketProvider()
	svc.httpClient.SetTransport(&redirectTransport{target: server.URL, wrapped: http.DefaultTransport})

	return svc
}

func bitbucketTestSettings() krci.GitServerSettings {
	return krci.GitServerSettings{Token: testBitbucketToken()}
}

// assertBitbucketTestAuth checks the request carries the credentials of bitbucketTestSettings.
func assertBitbucketTestAuth(t *testing.T, r *http.Request) {
	t.Helper()

	user, pass, ok := r.BasicAuth()
	assert.True(t, ok)
	assert.Equal(t, testBitbucketUser, user)
	assert.Equal(t, testBitbucketPass, pass)
}

func bitbucketRepoJSON(name, owner string) map[string]any {
	return map[string]any{
		"uuid":       "{" + name + "}",
		"name":       name,
		"mainbranch": map[string]any{"name": "main"},
		"owner":      map[string]any{"username": owner},
		"links":      map[string]any{"html": map[string]any{"href": "https://bitbucket.org/" + owner + "/" + name}},
	}
}

func wantRepository(name, owner string) models.Repository {
	return models.Repository{
		Id:            "{" + name + "}",
		Name:          name,
		Description:   pointer.To(""),
		DefaultBranch: pointer.To("main"),
		Owner:         pointer.To(owner),
		Url:           pointer.To("https://bitbucket.org/" + owner + "/" + name),
	}
}

func TestBitbucketServiceGetRepository(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /2.0/repositories/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		assertBitbucketTestAuth(t, r)
		writeBitbucketJSON(t, w, http.StatusOK, map[string]any{
			"uuid":        "{repo-uuid}",
			"name":        "repo",
			"description": "a repository",
			"mainbranch":  map[string]any{"name": "main"},
			"owner":       map[string]any{"username": "owner"},
			"links":       map[string]any{"html": map[string]any{"href": "https://bitbucket.org/owner/repo"}},
		})
	})

	svc := newBitbucketTestService(t, mux)

	got, err := svc.GetRepository(context.Background(), "owner", "repo", bitbucketTestSettings())

	require.NoError(t, err)
	assert.Equal(t, &models.Repository{
		Id:            "{repo-uuid}",
		Name:          "repo",
		Description:   pointer.To("a repository"),
		DefaultBranch: pointer.To("main"),
		Owner:         pointer.To("owner"),
		Url:           pointer.To("https://bitbucket.org/owner/repo"),
	}, got)
}

func TestBitbucketServiceGetRepositoryWithoutOptionalFields(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /2.0/repositories/owner/repo", func(w http.ResponseWriter, _ *http.Request) {
		writeBitbucketJSON(t, w, http.StatusOK, map[string]any{"uuid": "{repo-uuid}", "name": "repo"})
	})

	svc := newBitbucketTestService(t, mux)

	got, err := svc.GetRepository(context.Background(), "owner", "repo", bitbucketTestSettings())

	require.NoError(t, err)
	assert.Equal(t, &models.Repository{
		Id:            "{repo-uuid}",
		Name:          "repo",
		Description:   pointer.To(""),
		DefaultBranch: pointer.To(""),
		Owner:         pointer.To(""),
		Url:           pointer.To(""),
	}, got)
}

func TestBitbucketServiceListRepositories(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /2.0/repositories/acme", func(w http.ResponseWriter, r *http.Request) {
		assertBitbucketTestAuth(t, r)
		assert.Equal(t, `full_name ~ "api"`, r.URL.Query().Get("q"))

		if r.URL.Query().Get("page") == "2" {
			writeBitbucketJSON(t, w, http.StatusOK, map[string]any{
				"values": []any{bitbucketRepoJSON("api-gateway", "acme")},
			})

			return
		}

		writeBitbucketJSON(t, w, http.StatusOK, map[string]any{
			"values": []any{bitbucketRepoJSON("api", "acme"), bitbucketRepoJSON("api-docs", "acme")},
			"next":   `https://api.bitbucket.org/2.0/repositories/acme?page=2&q=full_name+~+"api"`,
		})
	})

	svc := newBitbucketTestService(t, mux)

	got, err := svc.ListRepositories(
		context.Background(), "acme", bitbucketTestSettings(), models.ListOptions{Name: pointer.To("api")},
	)

	require.NoError(t, err)
	assert.Equal(t, []models.Repository{
		wantRepository("api", "acme"),
		wantRepository("api-docs", "acme"),
		wantRepository("api-gateway", "acme"),
	}, got)
}

func TestBitbucketServiceListBranches(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /2.0/repositories/owner/repo/refs/branches", func(w http.ResponseWriter, r *http.Request) {
		assertBitbucketTestAuth(t, r)
		assert.Equal(t, "100", r.URL.Query().Get("pagelen"))

		if r.URL.Query().Get("page") == "2" {
			writeBitbucketJSON(t, w, http.StatusOK, map[string]any{
				"page":   2,
				"values": []any{map[string]any{"name": "feature"}},
			})

			return
		}

		writeBitbucketJSON(t, w, http.StatusOK, map[string]any{
			"page":   1,
			"values": []any{map[string]any{"name": "main"}, map[string]any{"name": "develop"}},
			"next":   "https://api.bitbucket.org/2.0/repositories/owner/repo/refs/branches?page=2",
		})
	})

	svc := newBitbucketTestService(t, mux)

	got, err := svc.ListBranches(context.Background(), "owner", "repo", bitbucketTestSettings(), models.ListOptions{})

	require.NoError(t, err)
	assert.Equal(t, []models.Branch{{Name: "main"}, {Name: "develop"}, {Name: "feature"}}, got)
}

type bitbucketClientCall struct {
	name        string
	path        string
	sentinelMsg string
	genericMsg  string
	call        func(*BitbucketService, krci.GitServerSettings) (any, error)
}

// bitbucketClientCalls lists every go-bitbucket backed method with the API path it requests.
func bitbucketClientCalls() []bitbucketClientCall {
	ctx := context.Background()

	return []bitbucketClientCall{
		{
			name:        "GetRepository",
			path:        "/2.0/repositories/owner/repo",
			sentinelMsg: "repository owner/repo",
			genericMsg:  "failed to get repository owner/repo",
			call: func(svc *BitbucketService, settings krci.GitServerSettings) (any, error) {
				return svc.GetRepository(ctx, "owner", "repo", settings)
			},
		},
		{
			name:        "ListRepositories",
			path:        "/2.0/repositories/acme",
			sentinelMsg: "account acme",
			genericMsg:  "failed to list repositories for account acme",
			call: func(svc *BitbucketService, settings krci.GitServerSettings) (any, error) {
				return svc.ListRepositories(ctx, "acme", settings, models.ListOptions{})
			},
		},
		{
			name:        "ListBranches",
			path:        "/2.0/repositories/owner/repo/refs/branches",
			sentinelMsg: "repository owner/repo",
			genericMsg:  "failed to list branches",
			call: func(svc *BitbucketService, settings krci.GitServerSettings) (any, error) {
				return svc.ListBranches(ctx, "owner", "repo", settings, models.ListOptions{})
			},
		},
	}
}

func TestBitbucketServiceClientCallsMapAPIErrors(t *testing.T) {
	for _, c := range bitbucketClientCalls() {
		for _, tt := range bitbucketErrorCases(c.sentinelMsg, c.sentinelMsg, c.genericMsg, bitbucketStatusDetail) {
			t.Run(c.name+"/"+tt.name, func(t *testing.T) {
				mux := http.NewServeMux()
				mux.HandleFunc("GET "+c.path, bitbucketErrorHandler(t, tt.status))

				got, err := c.call(newBitbucketTestService(t, mux), bitbucketTestSettings())

				assert.Nil(t, got)
				assertBitbucketError(t, err, tt)
			})
		}
	}
}

// Invalid-input tests serve an empty mux: a call that passes validation reaches it, not api.bitbucket.org.
func TestBitbucketServiceClientCallsRejectInvalidToken(t *testing.T) {
	for _, c := range bitbucketClientCalls() {
		t.Run(c.name, func(t *testing.T) {
			svc := newBitbucketTestService(t, http.NewServeMux())

			got, err := c.call(svc, krci.GitServerSettings{Token: "not-valid-base64!!!"})

			assert.Nil(t, got)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to decode bitbucket token")
		})
	}
}

func TestBitbucketServiceClientCallsRejectInvalidAPIBaseURL(t *testing.T) {
	for _, c := range bitbucketClientCalls() {
		t.Run(c.name, func(t *testing.T) {
			svc := newBitbucketTestService(t, http.NewServeMux())
			t.Setenv("BITBUCKET_API_BASE_URL", "://invalid")

			got, err := c.call(svc, bitbucketTestSettings())

			assert.Nil(t, got)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create bitbucket client")
		})
	}
}
