package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// newGitlabTestSettings starts a server for mux and returns settings that point the provider at it.
func newGitlabTestSettings(t *testing.T, mux *http.ServeMux) krci.GitServerSettings {
	t.Helper()

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return krci.GitServerSettings{Token: "test-token", Url: server.URL}
}

// pagedGitlabHandler serves pages[0] for a request without a page parameter and pages[n-1] for page=n,
// announcing the next page in X-Next-Page.
func pagedGitlabHandler(t *testing.T, pages ...any) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-token", r.Header.Get("PRIVATE-TOKEN"))

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

func gitlabProjectJSON(id int, path, namespace string) map[string]any {
	return map[string]any{
		"id":             id,
		"path":           path,
		"description":    path + " project",
		"default_branch": "main",
		"web_url":        "https://gitlab.example.com/" + namespace + "/" + path,
		"visibility":     "public",
		"namespace":      map[string]any{"full_path": namespace},
	}
}

func wantRepository(id, path, namespace string) models.Repository {
	return models.Repository{
		Id:            id,
		Name:          path,
		Description:   pointer.To(path + " project"),
		DefaultBranch: pointer.To("main"),
		Owner:         pointer.To(namespace),
		Url:           pointer.To("https://gitlab.example.com/" + namespace + "/" + path),
		Visibility:    pointer.To(models.RepositoryVisibilityPublic),
	}
}

func TestGitLabProviderListRepositoriesPagesThroughGroup(t *testing.T) {
	projects := pagedGitlabHandler(t,
		[]any{gitlabProjectJSON(1, "api", "acme"), gitlabProjectJSON(2, "api-docs", "acme")},
		[]any{gitlabProjectJSON(3, "api-gateway", "acme")},
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
		wantRepository("1", "api", "acme"),
		wantRepository("2", "api-docs", "acme"),
		wantRepository("3", "api-gateway", "acme"),
	}, got)
}
