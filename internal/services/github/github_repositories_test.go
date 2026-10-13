package github

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
)

func writeGitHubJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	assert.NoError(t, json.NewEncoder(w).Encode(body))
}

// pagedGitHubHandler serves pages[0] for a request without a page parameter and pages[n-1] for page=n,
// linking each page to the next one.
func pagedGitHubHandler(t *testing.T, pages ...any) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			_, err := fmt.Sscanf(p, "%d", &page)
			assert.NoError(t, err)
		}

		if page < len(pages) {
			w.Header().Set("Link", fmt.Sprintf(`<%s?page=%d>; rel="next"`, r.URL.Path, page+1))
		}

		writeGitHubJSON(t, w, http.StatusOK, pages[page-1])
	}
}

func gitHubErrorHandler(t *testing.T, status int) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, _ *http.Request) {
		writeGitHubJSON(t, w, status, map[string]string{"message": http.StatusText(status)})
	}
}

type gitHubErrorCase struct {
	name      string
	status    int
	wantErrIs error
	wantErr   string
}

// gitHubErrorCases covers the statuses mapped to domain sentinels and one passed through as a generic error.
func gitHubErrorCases(sentinelMsg, genericMsg string) []gitHubErrorCase {
	return []gitHubErrorCase{
		{name: "not found", status: http.StatusNotFound, wantErrIs: gferrors.ErrNotFound, wantErr: sentinelMsg},
		{name: "unauthorized", status: http.StatusUnauthorized, wantErrIs: gferrors.ErrUnauthorized, wantErr: sentinelMsg},
		{name: "other API error", status: http.StatusForbidden, wantErr: genericMsg},
	}
}

// assertGitHubError checks the wrapped message and that only the sentinel statuses map to a domain error.
func assertGitHubError(t *testing.T, err error, tc gitHubErrorCase) {
	t.Helper()

	require.Error(t, err)
	assert.Contains(t, err.Error(), tc.wantErr)

	if tc.wantErrIs != nil {
		assert.ErrorIs(t, err, tc.wantErrIs)

		return
	}

	assert.NotErrorIs(t, err, gferrors.ErrNotFound)
	assert.NotErrorIs(t, err, gferrors.ErrUnauthorized)
}

func newGitHubTestServer(t *testing.T, mux *http.ServeMux) *GitHubProvider {
	t.Helper()

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return newTestProvider(server.URL)
}

func gitHubRepo(id int64, name, owner string) map[string]any {
	return map[string]any{
		"id":       id,
		"name":     name,
		"owner":    map[string]any{"login": owner},
		"html_url": "https://github.com/" + owner + "/" + name,
	}
}

func publicRepository(id, name, owner string) models.Repository {
	return models.Repository{
		Id:         id,
		Name:       name,
		Owner:      ptr(owner),
		Url:        ptr("https://github.com/" + owner + "/" + name),
		Visibility: ptr(models.RepositoryVisibilityPublic),
	}
}

func TestGitHubProviderGetRepository(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		writeGitHubJSON(t, w, http.StatusOK, map[string]any{
			"id":             int64(42),
			"name":           "repo",
			"owner":          map[string]any{"login": "owner"},
			"html_url":       "https://github.com/owner/repo",
			"default_branch": "main",
			"description":    "a repository",
			"private":        true,
		})
	})

	provider := newGitHubTestServer(t, mux)

	got, err := provider.GetRepository(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: "test-token"},
	)

	require.NoError(t, err)
	assert.Equal(t, &models.Repository{
		Id:            "42",
		Name:          "repo",
		Owner:         ptr("owner"),
		Url:           ptr("https://github.com/owner/repo"),
		DefaultBranch: ptr("main"),
		Description:   ptr("a repository"),
		Visibility:    ptr(models.RepositoryVisibilityPrivate),
	}, got)
}

func TestGitHubProviderGetRepositoryWithoutOptionalFields(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo", func(w http.ResponseWriter, _ *http.Request) {
		writeGitHubJSON(t, w, http.StatusOK, map[string]any{"id": int64(7)})
	})

	provider := newGitHubTestServer(t, mux)

	got, err := provider.GetRepository(context.Background(), "owner", "repo", krci.GitServerSettings{})

	require.NoError(t, err)
	assert.Equal(t, &models.Repository{
		Id:         "7",
		Owner:      ptr(""),
		Url:        ptr(""),
		Visibility: ptr(models.RepositoryVisibilityPublic),
	}, got)
}

func TestGitHubProviderGetRepositoryErrors(t *testing.T) {
	tests := gitHubErrorCases("repository owner/repo", "failed to get repository owner/repo")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /repos/owner/repo", gitHubErrorHandler(t, tt.status))

			provider := newGitHubTestServer(t, mux)

			got, err := provider.GetRepository(context.Background(), "owner", "repo", krci.GitServerSettings{})

			assert.Nil(t, got)
			assertGitHubError(t, err, tt)
		})
	}
}

func TestGitHubProviderListRepositoriesForOrganization(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /orgs/acme", func(w http.ResponseWriter, _ *http.Request) {
		writeGitHubJSON(t, w, http.StatusOK, map[string]any{"login": "acme"})
	})
	mux.HandleFunc("GET /orgs/acme/repos", pagedGitHubHandler(t,
		[]any{gitHubRepo(1, "api", "acme"), gitHubRepo(2, "web", "acme")},
		[]any{gitHubRepo(3, "docs", "acme")},
	))

	provider := newGitHubTestServer(t, mux)

	got, err := provider.ListRepositories(context.Background(), "acme", krci.GitServerSettings{}, models.ListOptions{})

	require.NoError(t, err)
	assert.Equal(t, []models.Repository{
		publicRepository("1", "api", "acme"),
		publicRepository("2", "web", "acme"),
		publicRepository("3", "docs", "acme"),
	}, got)
}

func TestGitHubProviderListRepositoriesForUser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /orgs/jdoe", gitHubErrorHandler(t, http.StatusNotFound))
	mux.HandleFunc("GET /users/jdoe/repos", pagedGitHubHandler(t,
		[]any{gitHubRepo(1, "Dotfiles", "jdoe"), gitHubRepo(2, "blog", "jdoe")},
		[]any{gitHubRepo(3, "old-dotfiles", "jdoe")},
	))

	provider := newGitHubTestServer(t, mux)

	got, err := provider.ListRepositories(
		context.Background(),
		"jdoe",
		krci.GitServerSettings{},
		models.ListOptions{Name: ptr("dotfiles")},
	)

	require.NoError(t, err)
	assert.Equal(t, []models.Repository{
		publicRepository("1", "Dotfiles", "jdoe"),
		publicRepository("3", "old-dotfiles", "jdoe"),
	}, got)
}

func TestGitHubProviderListRepositoriesErrors(t *testing.T) {
	tests := gitHubErrorCases("organization or user ghost", "failed to list repositories for org ghost")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /orgs/ghost", gitHubErrorHandler(t, http.StatusNotFound))
			mux.HandleFunc("GET /users/ghost/repos", gitHubErrorHandler(t, tt.status))

			provider := newGitHubTestServer(t, mux)

			got, err := provider.ListRepositories(context.Background(), "ghost", krci.GitServerSettings{}, models.ListOptions{})

			assert.Nil(t, got)
			assertGitHubError(t, err, tt)
		})
	}
}

func TestGitHubProviderListBranches(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo/branches", pagedGitHubHandler(t,
		[]any{map[string]any{"name": "main"}, map[string]any{"name": "develop"}},
		[]any{map[string]any{"name": "feature"}},
	))

	provider := newGitHubTestServer(t, mux)

	got, err := provider.ListBranches(
		context.Background(), "owner", "repo", krci.GitServerSettings{}, models.ListOptions{},
	)

	require.NoError(t, err)
	assert.Equal(t, []models.Branch{{Name: "main"}, {Name: "develop"}, {Name: "feature"}}, got)
}

func TestGitHubProviderListBranchesErrors(t *testing.T) {
	tests := gitHubErrorCases("repository owner/repo", "failed to list branches")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /repos/owner/repo/branches", gitHubErrorHandler(t, tt.status))

			provider := newGitHubTestServer(t, mux)

			got, err := provider.ListBranches(
				context.Background(), "owner", "repo", krci.GitServerSettings{}, models.ListOptions{},
			)

			assert.Nil(t, got)
			assertGitHubError(t, err, tt)
		})
	}
}

func currentGitHubUserHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, _ *http.Request) {
		writeGitHubJSON(t, w, http.StatusOK, map[string]any{
			"id":         int64(100),
			"login":      "jdoe",
			"avatar_url": "https://avatars.example.com/jdoe",
		})
	}
}

func TestGitHubProviderListUserOrganizations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /user", currentGitHubUserHandler(t))
	mux.HandleFunc("GET /user/memberships/orgs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "active", r.URL.Query().Get("state"))
		pagedGitHubHandler(t,
			[]any{
				map[string]any{"organization": map[string]any{
					"id": int64(1), "login": "acme", "avatar_url": "https://avatars.example.com/acme",
				}},
				map[string]any{"state": "active"},
			},
			[]any{
				map[string]any{"organization": map[string]any{"id": int64(2), "login": "globex"}},
			},
		)(w, r)
	})

	provider := newGitHubTestServer(t, mux)

	got, err := provider.ListUserOrganizations(context.Background(), krci.GitServerSettings{})

	require.NoError(t, err)
	assert.Equal(t, []models.Organization{
		{Id: "1", Name: "acme", AvatarUrl: ptr("https://avatars.example.com/acme")},
		{Id: "2", Name: "globex"},
		{Id: "100", Name: "jdoe", AvatarUrl: ptr("https://avatars.example.com/jdoe")},
	}, got)
}

func TestGitHubProviderListUserOrganizationsErrors(t *testing.T) {
	healthyMemberships := func(t *testing.T) http.HandlerFunc {
		return pagedGitHubHandler(t, []any{
			map[string]any{"organization": map[string]any{"id": int64(1), "login": "acme"}},
		})
	}

	tests := []struct {
		name        string
		user        func(t *testing.T) http.HandlerFunc
		memberships func(t *testing.T) http.HandlerFunc
		wantErr     string
	}{
		{
			name:        "current user request fails",
			user:        func(t *testing.T) http.HandlerFunc { return gitHubErrorHandler(t, http.StatusUnauthorized) },
			memberships: healthyMemberships,
			wantErr:     "failed to get current user",
		},
		{
			name:        "organization memberships request fails",
			user:        currentGitHubUserHandler,
			memberships: func(t *testing.T) http.HandlerFunc { return gitHubErrorHandler(t, http.StatusForbidden) },
			wantErr:     "failed to list organizations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /user", tt.user(t))
			mux.HandleFunc("GET /user/memberships/orgs", tt.memberships(t))

			provider := newGitHubTestServer(t, mux)

			got, err := provider.ListUserOrganizations(context.Background(), krci.GitServerSettings{})

			require.Error(t, err)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
