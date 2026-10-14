package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-github/v72/github"
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
// msg is the prefix every case expects.
func gitHubErrorCases(msg string) []gitHubErrorCase {
	return []gitHubErrorCase{
		{name: "not found", status: http.StatusNotFound, wantErrIs: gferrors.ErrNotFound, wantErr: msg},
		{name: "unauthorized", status: http.StatusUnauthorized, wantErrIs: gferrors.ErrUnauthorized, wantErr: msg},
		{name: "forbidden", status: http.StatusForbidden, wantErrIs: gferrors.ErrUnauthorized, wantErr: msg},
		{name: "other API error", status: http.StatusInternalServerError, wantErr: msg},
	}
}

// assertGitHubError checks the wrapped message, the upstream error text, and that only the sentinel statuses
// map to a domain error.
func assertGitHubError(t *testing.T, err error, tc gitHubErrorCase) {
	t.Helper()

	require.Error(t, err)
	assert.Contains(t, err.Error(), tc.wantErr)
	assert.Contains(t, err.Error(), fmt.Sprintf("%d %s", tc.status, http.StatusText(tc.status)))

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
	tests := gitHubErrorCases("failed to get repository owner/repo")

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

func TestGitHubProviderGetRepositoryRateLimited(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
		writeGitHubJSON(t, w, http.StatusForbidden, map[string]string{"message": "API rate limit exceeded"})
	})

	provider := newGitHubTestServer(t, mux)

	got, err := provider.GetRepository(context.Background(), "owner", "repo", krci.GitServerSettings{})

	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get repository owner/repo")
	assert.NotErrorIs(t, err, gferrors.ErrUnauthorized)
	assert.NotErrorIs(t, err, gferrors.ErrNotFound)

	var rateLimitErr *github.RateLimitError

	assert.ErrorAs(t, err, &rateLimitErr)
}

func TestGitHubProviderGetRepositorySecondaryRateLimited(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo", func(w http.ResponseWriter, _ *http.Request) {
		writeGitHubJSON(t, w, http.StatusForbidden, map[string]string{
			"message": "You have exceeded a secondary rate limit",
			"documentation_url": "https://docs.github.com/rest/overview/" +
				"rate-limits-for-the-rest-api#about-secondary-rate-limits",
		})
	})

	provider := newGitHubTestServer(t, mux)

	got, err := provider.GetRepository(context.Background(), "owner", "repo", krci.GitServerSettings{})

	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get repository owner/repo")
	assert.NotErrorIs(t, err, gferrors.ErrUnauthorized)
	assert.NotErrorIs(t, err, gferrors.ErrNotFound)

	var abuseErr *github.AbuseRateLimitError

	assert.ErrorAs(t, err, &abuseErr)
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
	tests := gitHubErrorCases("failed to list repositories for org ghost")

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

// Only a 404 on the organization lookup falls back to the user listing; any other lookup error fails the list.
func TestGitHubProviderListRepositoriesOrganizationLookupErrors(t *testing.T) {
	tests := slices.DeleteFunc(
		gitHubErrorCases("failed to list repositories for org acme"),
		func(tc gitHubErrorCase) bool { return tc.status == http.StatusNotFound },
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userListCalls atomic.Int32

			mux := http.NewServeMux()
			mux.HandleFunc("GET /orgs/acme", gitHubErrorHandler(t, tt.status))
			mux.HandleFunc("GET /users/acme/repos", func(w http.ResponseWriter, _ *http.Request) {
				userListCalls.Add(1)
				writeGitHubJSON(t, w, http.StatusOK, []any{gitHubRepo(1, "api", "acme")})
			})

			provider := newGitHubTestServer(t, mux)

			got, err := provider.ListRepositories(context.Background(), "acme", krci.GitServerSettings{}, models.ListOptions{})

			assert.Nil(t, got)
			assertGitHubError(t, err, tt)
			assert.Zero(t, userListCalls.Load())
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
	tests := gitHubErrorCases("failed to list branches")

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
	endpoints := []struct {
		name     string
		failUser bool
		msg      string
	}{
		{name: "current user", failUser: true, msg: "failed to get current user"},
		{name: "organization memberships", msg: "failed to list organizations"},
	}

	for _, endpoint := range endpoints {
		for _, tt := range gitHubErrorCases(endpoint.msg) {
			t.Run(endpoint.name+"/"+tt.name, func(t *testing.T) {
				user := currentGitHubUserHandler(t)
				memberships := pagedGitHubHandler(t, []any{
					map[string]any{"organization": map[string]any{"id": int64(1), "login": "acme"}},
				})

				if endpoint.failUser {
					user = gitHubErrorHandler(t, tt.status)
				} else {
					memberships = gitHubErrorHandler(t, tt.status)
				}

				mux := http.NewServeMux()
				mux.HandleFunc("GET /user", user)
				mux.HandleFunc("GET /user/memberships/orgs", memberships)

				provider := newGitHubTestServer(t, mux)

				got, err := provider.ListUserOrganizations(context.Background(), krci.GitServerSettings{})

				assert.Nil(t, got)
				assertGitHubError(t, err, tt)
			})
		}
	}
}
