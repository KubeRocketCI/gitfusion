package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gitlab "gitlab.com/gitlab-org/api/client-go"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	"github.com/KubeRocketCI/gitfusion/pkg/pointer"
)

func writeGitlabJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	assert.NoError(t, json.NewEncoder(w).Encode(body))
}

// gitlabTestContext bounds a test so a pagination loop that never ends fails instead of hanging.
func gitlabTestContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	return ctx
}

const testGitlabToken = "test-token"

// assertGitlabTestAuth checks the request carries the token of newGitlabTestSettings.
func assertGitlabTestAuth(t *testing.T, r *http.Request) {
	t.Helper()

	assert.Equal(t, testGitlabToken, r.Header.Get("PRIVATE-TOKEN"))
}

// newGitlabTestSettings starts a server for mux and returns settings that point the provider at it.
func newGitlabTestSettings(t *testing.T, mux *http.ServeMux) krci.GitServerSettings {
	t.Helper()

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return krci.GitServerSettings{Token: testGitlabToken, Url: server.URL}
}

// pagedGitlabHandler serves pages[0] for a request without a page parameter and pages[n-1] for page=n,
// announcing the next page in X-Next-Page.
func pagedGitlabHandler(t *testing.T, pages ...any) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		assertGitlabTestAuth(t, r)

		page := 1

		if p := r.URL.Query().Get("page"); p != "" {
			var err error

			page, err = strconv.Atoi(p)
			assert.NoError(t, err)
		}

		if page < len(pages) {
			w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
		}

		writeGitlabJSON(t, w, http.StatusOK, pages[page-1])
	}
}

func gitlabErrorHandler(t *testing.T, status int) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, _ *http.Request) {
		writeGitlabJSON(t, w, status, map[string]string{"message": http.StatusText(status)})
	}
}

type gitlabErrorCase struct {
	name      string
	status    int
	wantErrIs error
	wantErr   string
}

// assertGitlabResponseStatus checks that err wraps a go-gitlab error response with the given HTTP status.
func assertGitlabResponseStatus(t *testing.T, err error, status int) {
	t.Helper()

	var errResp *gitlab.ErrorResponse

	require.ErrorAs(t, err, &errResp)
	require.NotNil(t, errResp.Response)
	assert.Equal(t, status, errResp.Response.StatusCode)
}

// assertGitlabError checks that err contains tc.wantErr and matches tc.wantErrIs.
// ErrNotFound must also wrap gitlab.ErrNotFound; ErrUnauthorized must also wrap the go-gitlab error response with
// status tc.status. With no wantErrIs the error matches neither ErrNotFound nor ErrUnauthorized.
func assertGitlabError(t *testing.T, err error, tc gitlabErrorCase) {
	t.Helper()

	require.Error(t, err)
	assert.Contains(t, err.Error(), tc.wantErr)

	if tc.wantErrIs == nil {
		assert.NotErrorIs(t, err, gferrors.ErrNotFound)
		assert.NotErrorIs(t, err, gferrors.ErrUnauthorized)

		return
	}

	assert.ErrorIs(t, err, tc.wantErrIs)

	switch {
	case errors.Is(tc.wantErrIs, gferrors.ErrNotFound):
		assert.ErrorIs(t, err, gitlab.ErrNotFound)
	case errors.Is(tc.wantErrIs, gferrors.ErrUnauthorized):
		assertGitlabResponseStatus(t, err, tc.status)
	}
}

// gitlabProjectJSON returns a public project in the acme group.
func gitlabProjectJSON(id int, path string) map[string]any {
	return map[string]any{
		"id":             id,
		"path":           path,
		"description":    path + " project",
		"default_branch": "main",
		"web_url":        "https://gitlab.example.com/acme/" + path,
		"visibility":     "public",
		"namespace":      map[string]any{"full_path": "acme"},
	}
}

// wantRepository is the model of gitlabProjectJSON.
func wantRepository(id, path string) models.Repository {
	return models.Repository{
		Id:            id,
		Name:          path,
		Description:   pointer.To(path + " project"),
		DefaultBranch: pointer.To("main"),
		Owner:         pointer.To("acme"),
		Url:           pointer.To("https://gitlab.example.com/acme/" + path),
		Visibility:    pointer.To(models.RepositoryVisibilityPublic),
	}
}

func TestGitLabProviderListRepositoriesPagesThroughGroup(t *testing.T) {
	projects := pagedGitlabHandler(t,
		[]any{gitlabProjectJSON(1, "api"), gitlabProjectJSON(2, "api-docs")},
		[]any{gitlabProjectJSON(3, "api-gateway")},
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v4/groups/acme/projects", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		assert.Equal(t, "api", r.URL.Query().Get("search"))
		projects(w, r)
	})

	got, err := NewGitlabProvider().ListRepositories(
		gitlabTestContext(t), "acme", newGitlabTestSettings(t, mux), models.ListOptions{Name: pointer.To("api")},
	)

	require.NoError(t, err)
	assert.Equal(t, []models.Repository{
		wantRepository("1", "api"),
		wantRepository("2", "api-docs"),
		wantRepository("3", "api-gateway"),
	}, got)
}

func TestGitLabProviderGetRepository(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v4/projects/acme%2Fapi", func(w http.ResponseWriter, r *http.Request) {
		assertGitlabTestAuth(t, r)

		project := gitlabProjectJSON(1, "api")
		project["visibility"] = "private"
		writeGitlabJSON(t, w, http.StatusOK, project)
	})

	got, err := NewGitlabProvider().GetRepository(gitlabTestContext(t), "acme", "api", newGitlabTestSettings(t, mux))

	require.NoError(t, err)

	want := wantRepository("1", "api")
	want.Visibility = pointer.To(models.RepositoryVisibilityPrivate)
	assert.Equal(t, &want, got)
}

func TestGitLabProviderListBranches(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v4/projects/acme%2Fapi/repository/branches", pagedGitlabHandler(t,
		[]any{map[string]any{"name": "main"}, map[string]any{"name": "develop"}},
		[]any{map[string]any{"name": "feature"}},
	))

	got, err := NewGitlabProvider().ListBranches(
		gitlabTestContext(t), "acme", "api", newGitlabTestSettings(t, mux), models.ListOptions{},
	)

	require.NoError(t, err)
	assert.Equal(t, []models.Branch{{Name: "main"}, {Name: "develop"}, {Name: "feature"}}, got)
}

func TestGitLabProviderListUserOrganizations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v4/groups", pagedGitlabHandler(t,
		[]any{
			map[string]any{"id": 1, "full_path": "acme", "avatar_url": "https://gitlab.example.com/acme.png"},
			map[string]any{"id": 2, "full_path": "acme/platform"},
		},
		[]any{map[string]any{"id": 3, "full_path": "globex"}},
	))

	got, err := NewGitlabProvider().ListUserOrganizations(gitlabTestContext(t), newGitlabTestSettings(t, mux))

	require.NoError(t, err)
	assert.Equal(t, []models.Organization{
		{Id: "1", Name: "acme", AvatarUrl: pointer.To("https://gitlab.example.com/acme.png")},
		{Id: "2", Name: "acme/platform"},
		{Id: "3", Name: "globex"},
	}, got)
}

func TestGitLabProviderTriggerPipeline(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		want     *models.PipelineResponse
	}{
		{
			name: "with commit SHA",
			response: map[string]any{
				"id": 42, "web_url": "https://gitlab.example.com/acme/api/-/pipelines/42",
				"status": "created", "ref": "main", "sha": "abc123",
			},
			want: &models.PipelineResponse{
				Id: 42, WebUrl: "https://gitlab.example.com/acme/api/-/pipelines/42",
				Status: "created", Ref: "main", Sha: pointer.To("abc123"),
			},
		},
		{
			name: "without commit SHA",
			response: map[string]any{
				"id": 43, "web_url": "https://gitlab.example.com/acme/api/-/pipelines/43",
				"status": "pending", "ref": "main",
			},
			want: &models.PipelineResponse{
				Id: 43, WebUrl: "https://gitlab.example.com/acme/api/-/pipelines/43",
				Status: "pending", Ref: "main",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/v4/projects/acme%2Fapi/pipeline", func(w http.ResponseWriter, r *http.Request) {
				assertGitlabTestAuth(t, r)

				var body struct {
					Ref       string              `json:"ref"`
					Variables []map[string]string `json:"variables"`
				}

				assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, "main", body.Ref)
				assert.Equal(t, []map[string]string{
					{"key": "DEPLOY", "value": "true"},
					{"key": "CONFIG", "value": "a: b", "variable_type": "file"},
				}, body.Variables)
				writeGitlabJSON(t, w, http.StatusCreated, tt.response)
			})

			got, err := NewGitlabProvider().TriggerPipeline(
				gitlabTestContext(t),
				"acme/api",
				"main",
				[]models.PipelineVariable{
					{Key: "DEPLOY", Value: "true"},
					{Key: "CONFIG", Value: "a: b", VariableType: pointer.To(models.File)},
				},
				newGitlabTestSettings(t, mux),
			)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// gitlabBadRequest is how go-gitlab reports the 400 response of gitlabErrorHandler.
const gitlabBadRequest = "400 {message: Bad Request}"

type gitlabCall struct {
	name  string
	route string
	cases []gitlabErrorCase
	call  func(context.Context, krci.GitServerSettings) (any, error)
}

// genericGitlabError is the case for a 400 response: the error contains wantErr and matches no domain error.
func genericGitlabError(wantErr string) gitlabErrorCase {
	return gitlabErrorCase{name: "other API error", status: http.StatusBadRequest, wantErr: wantErr}
}

// gitlabRejectedCredentials is the case for a 401 or 403 response that maps to ErrUnauthorized.
func gitlabRejectedCredentials(status int, wantErr string) gitlabErrorCase {
	return gitlabErrorCase{
		name: http.StatusText(status), status: status, wantErrIs: gferrors.ErrUnauthorized, wantErr: wantErr,
	}
}

// gitlabNotFound is the case for a 404 response that maps to ErrNotFound.
func gitlabNotFound(wantErr string) gitlabErrorCase {
	return gitlabErrorCase{
		name: "not found", status: http.StatusNotFound, wantErrIs: gferrors.ErrNotFound, wantErr: wantErr,
	}
}

// gitlabCalls lists the provider methods under test with the route each requests and its error mapping.
func gitlabCalls() []gitlabCall {
	p := NewGitlabProvider()

	return []gitlabCall{
		{
			name:  "GetRepository",
			route: "GET /api/v4/projects/acme%2Fapi",
			cases: []gitlabErrorCase{
				gitlabNotFound("repository acme/api"),
				gitlabRejectedCredentials(http.StatusUnauthorized, "repository acme/api"),
				gitlabRejectedCredentials(http.StatusForbidden, "repository acme/api"),
				genericGitlabError(gitlabBadRequest),
			},
			call: func(ctx context.Context, settings krci.GitServerSettings) (any, error) {
				return p.GetRepository(ctx, "acme", "api", settings)
			},
		},
		{
			name:  "ListRepositories",
			route: "GET /api/v4/groups/acme/projects",
			cases: []gitlabErrorCase{
				gitlabNotFound("owner acme"),
				gitlabRejectedCredentials(http.StatusUnauthorized, "owner acme"),
				gitlabRejectedCredentials(http.StatusForbidden, "owner acme"),
				genericGitlabError(gitlabBadRequest),
			},
			call: func(ctx context.Context, settings krci.GitServerSettings) (any, error) {
				return p.ListRepositories(ctx, "acme", settings, models.ListOptions{})
			},
		},
		{
			name:  "ListBranches",
			route: "GET /api/v4/projects/acme%2Fapi/repository/branches",
			cases: []gitlabErrorCase{
				gitlabNotFound("project acme/api"),
				gitlabRejectedCredentials(http.StatusUnauthorized, "project acme/api"),
				gitlabRejectedCredentials(http.StatusForbidden, "project acme/api"),
				genericGitlabError("failed to list branches for acme/api"),
			},
			call: func(ctx context.Context, settings krci.GitServerSettings) (any, error) {
				return p.ListBranches(ctx, "acme", "api", settings, models.ListOptions{})
			},
		},
		{
			name:  "ListUserOrganizations",
			route: "GET /api/v4/groups",
			cases: []gitlabErrorCase{
				gitlabNotFound("groups"),
				gitlabRejectedCredentials(http.StatusUnauthorized, "groups"),
				gitlabRejectedCredentials(http.StatusForbidden, "groups"),
				genericGitlabError("failed to list groups"),
			},
			call: func(ctx context.Context, settings krci.GitServerSettings) (any, error) {
				return p.ListUserOrganizations(ctx, settings)
			},
		},
		{
			name:  "TriggerPipeline",
			route: "POST /api/v4/projects/acme%2Fapi/pipeline",
			cases: []gitlabErrorCase{
				gitlabNotFound("project acme/api or ref main"),
				gitlabRejectedCredentials(http.StatusUnauthorized, "project acme/api or ref main"),
				gitlabRejectedCredentials(http.StatusForbidden, "project acme/api or ref main"),
				genericGitlabError("create pipeline for acme/api ref main"),
			},
			call: func(ctx context.Context, settings krci.GitServerSettings) (any, error) {
				return p.TriggerPipeline(ctx, "acme/api", "main", nil, settings)
			},
		},
		{
			name:  "ListPipelineJobs",
			route: "GET /api/v4/projects/acme%2Fapi/pipelines/7/jobs",
			cases: []gitlabErrorCase{
				gitlabNotFound("project acme/api or id 7"),
				gitlabRejectedCredentials(http.StatusUnauthorized, "project acme/api or id 7"),
				gitlabRejectedCredentials(http.StatusForbidden, "project acme/api or id 7"),
				genericGitlabError("gitlab jobs request failed for acme/api id 7"),
			},
			call: func(ctx context.Context, settings krci.GitServerSettings) (any, error) {
				return p.ListPipelineJobs(ctx, "acme/api", 7, settings)
			},
		},
	}
}

func TestGitLabProviderCallsMapAPIErrors(t *testing.T) {
	for _, c := range gitlabCalls() {
		for _, tt := range c.cases {
			t.Run(c.name+"/"+tt.name, func(t *testing.T) {
				mux := http.NewServeMux()
				mux.HandleFunc(c.route, gitlabErrorHandler(t, tt.status))

				got, err := c.call(gitlabTestContext(t), newGitlabTestSettings(t, mux))

				assert.Nil(t, got)
				assertGitlabError(t, err, tt)
			})
		}
	}
}

func TestGitLabProviderCallsRejectInvalidServerURL(t *testing.T) {
	for _, c := range gitlabCalls() {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.call(gitlabTestContext(t), krci.GitServerSettings{Token: testGitlabToken, Url: "://invalid"})

			assert.Nil(t, got)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing protocol scheme")
		})
	}
}
