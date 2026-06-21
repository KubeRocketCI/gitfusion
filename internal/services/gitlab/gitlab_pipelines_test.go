package gitlab

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

func TestNormalizeGitLabPipelineStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   models.PipelineStatus
	}{
		{name: "pending", status: "pending", want: models.PipelineStatusPending},
		{name: "created maps to pending", status: "created", want: models.PipelineStatusPending},
		{name: "waiting_for_resource maps to pending", status: "waiting_for_resource", want: models.PipelineStatusPending},
		{name: "preparing maps to pending", status: "preparing", want: models.PipelineStatusPending},
		{name: "running", status: "running", want: models.PipelineStatusRunning},
		{name: "success", status: "success", want: models.PipelineStatusSuccess},
		{name: "failed", status: "failed", want: models.PipelineStatusFailed},
		{name: "canceled maps to cancelled", status: "canceled", want: models.PipelineStatusCancelled},
		{name: "skipped", status: "skipped", want: models.PipelineStatusSkipped},
		{name: "manual", status: "manual", want: models.PipelineStatusManual},
		{name: "scheduled maps to manual", status: "scheduled", want: models.PipelineStatusManual},
		{name: "unknown defaults to pending", status: "unknown", want: models.PipelineStatusPending},
		{name: "empty defaults to pending", status: "", want: models.PipelineStatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitLabPipelineStatus(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeGitLabPipelineSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   models.PipelineSource
	}{
		{name: "push", source: "push", want: models.PipelineSourcePush},
		{name: "merge_request_event", source: "merge_request_event", want: models.PipelineSourceMergeRequest},
		{name: "schedule", source: "schedule", want: models.PipelineSourceSchedule},
		{name: "web maps to manual", source: "web", want: models.PipelineSourceManual},
		{name: "chat maps to manual", source: "chat", want: models.PipelineSourceManual},
		{name: "trigger", source: "trigger", want: models.PipelineSourceTrigger},
		{name: "pipeline maps to trigger", source: "pipeline", want: models.PipelineSourceTrigger},
		{name: "api maps to trigger", source: "api", want: models.PipelineSourceTrigger},
		{name: "unknown maps to other", source: "unknown", want: models.PipelineSourceOther},
		{name: "empty maps to other", source: "", want: models.PipelineSourceOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitLabPipelineSource(tt.source)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapPipelineStatusToGitLab(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		wantNil bool
	}{
		{name: "pending maps", status: "pending", wantNil: false},
		{name: "running maps", status: "running", wantNil: false},
		{name: "success maps", status: "success", wantNil: false},
		{name: "failed maps", status: "failed", wantNil: false},
		{name: "cancelled maps", status: "cancelled", wantNil: false},
		{name: "skipped maps", status: "skipped", wantNil: false},
		{name: "manual maps", status: "manual", wantNil: false},
		{name: "unknown returns nil", status: "unknown", wantNil: true},
		{name: "empty returns nil", status: "", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPipelineStatusToGitLab(tt.status)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestGitLabProviderListPipelinesFieldMapping(t *testing.T) {
	pipelinesJSON := `[
		{
			"id": 100,
			"iid": 5,
			"project_id": 42,
			"status": "success",
			"source": "push",
			"ref": "main",
			"sha": "abc123def456",
			"web_url": "https://gitlab.com/project/-/pipelines/100",
			"created_at": "2026-01-15T10:30:00.000Z",
			"updated_at": "2026-01-16T14:00:00.000Z"
		}
	]`

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/pipelines", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(pipelinesJSON))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()
	page := 1
	perPage := 20

	result, err := provider.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
		models.PipelineListOptions{Page: page, PerPage: perPage},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)

	p := result.Data[0]
	assert.Equal(t, "100", p.Id)
	assert.Equal(t, models.PipelineStatusSuccess, p.Status)
	assert.Equal(t, "main", p.Ref)
	assert.Equal(t, "abc123def456", p.Sha)
	assert.Equal(t, "https://gitlab.com/project/-/pipelines/100", p.WebUrl)

	// Source mapping
	require.NotNil(t, p.Source)
	assert.Equal(t, models.PipelineSourcePush, *p.Source)

	// ProjectId mapping
	require.NotNil(t, p.ProjectId)
	assert.Equal(t, "42", *p.ProjectId)

	// Pagination
	assert.Equal(t, 1, result.Pagination.Total)
	require.NotNil(t, result.Pagination.Page)
	assert.Equal(t, page, *result.Pagination.Page)
	require.NotNil(t, result.Pagination.PerPage)
	assert.Equal(t, perPage, *result.Pagination.PerPage)
}

func TestGitLabProviderListPipelinesMultipleStatuses(t *testing.T) {
	pipelinesJSON := `[
		{
			"id": 1, "status": "running", "source": "push",
			"ref": "main", "sha": "aaa",
			"web_url": "https://gitlab.com/p/1",
			"created_at": "2026-01-15T10:00:00.000Z"
		},
		{
			"id": 2, "status": "failed",
			"source": "merge_request_event",
			"ref": "feature", "sha": "bbb",
			"web_url": "https://gitlab.com/p/2",
			"created_at": "2026-01-15T11:00:00.000Z"
		},
		{
			"id": 3, "status": "canceled", "source": "web",
			"ref": "main", "sha": "ccc",
			"web_url": "https://gitlab.com/p/3",
			"created_at": "2026-01-15T12:00:00.000Z"
		}
	]`

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/pipelines", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total", "3")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(pipelinesJSON))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()
	result, err := provider.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.Len(t, result.Data, 3)

	assert.Equal(t, models.PipelineStatusRunning, result.Data[0].Status)
	assert.Equal(t, models.PipelineStatusFailed, result.Data[1].Status)
	assert.Equal(t, models.PipelineStatusCancelled, result.Data[2].Status)

	require.NotNil(t, result.Data[0].Source)
	assert.Equal(t, models.PipelineSourcePush, *result.Data[0].Source)
	require.NotNil(t, result.Data[1].Source)
	assert.Equal(t, models.PipelineSourceMergeRequest, *result.Data[1].Source)
	require.NotNil(t, result.Data[2].Source)
	assert.Equal(t, models.PipelineSourceManual, *result.Data[2].Source)
}

func TestGitLabProviderListPipelinesFilterParams(t *testing.T) {
	pipelinesJSON := `[
		{
			"id": 100,
			"status": "success",
			"source": "push",
			"ref": "main",
			"sha": "abc123",
			"web_url": "https://gitlab.com/project/-/pipelines/100",
			"created_at": "2026-01-15T10:30:00.000Z"
		}
	]`

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/pipelines", func(w http.ResponseWriter, r *http.Request) {
		// Assert that filter query parameters are correctly forwarded
		assert.Equal(t, "main", r.URL.Query().Get("ref"))
		assert.Equal(t, "success", r.URL.Query().Get("status"))

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(pipelinesJSON))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()

	ref := "main"
	status := "success"

	result, err := provider.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
		models.PipelineListOptions{
			Ref:     &ref,
			Status:  &status,
			Page:    1,
			PerPage: 20,
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "100", result.Data[0].Id)
}

func TestGitLabProviderListPipelinesNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/nonexistent%2Fproject/pipelines", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Project Not Found"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()
	result, err := provider.ListPipelines(
		context.Background(),
		"nonexistent/project",
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestGitLabProviderListPipelinesUnauthorized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/pipelines", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"401 Unauthorized"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()
	result, err := provider.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: "bad-token", Url: server.URL},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unauthorized")
}

// --- ListPipelineJobs tests ---

func TestGitLabProviderListPipelineJobsAscendingNumericOrder(t *testing.T) {
	// IDs are intentionally out of lexicographic order to assert numeric sort (2,9,10 not 10,2,9).
	jobsJSON := `[
		{"id": 10, "name": "deploy", "stage": "deploy", "status": "success", "web_url": "https://gitlab.com/j/10"},
		{"id": 2,  "name": "test",   "stage": "test",   "status": "success", "web_url": "https://gitlab.com/j/2"},
		{"id": 9,  "name": "build",  "stage": "build",  "status": "success", "web_url": "https://gitlab.com/j/9"}
	]`

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/pipelines/42/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jobsJSON))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()

	result, err := provider.ListPipelineJobs(
		context.Background(),
		"owner/repo",
		42,
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
	)

	require.NoError(t, err)
	require.Len(t, result, 3)

	// Must be numeric ascending: 2, 9, 10 — NOT lexicographic 10, 2, 9.
	assert.Equal(t, "2", result[0].Id)
	assert.Equal(t, "9", result[1].Id)
	assert.Equal(t, "10", result[2].Id)
}

func TestGitLabProviderListPipelineJobsFieldMapping(t *testing.T) {
	jobsJSON := `[
		{
			"id": 5,
			"name": "build",
			"stage": "build",
			"status": "success",
			"ref": "main",
			"web_url": "https://gitlab.com/project/-/jobs/5",
			"allow_failure": false,
			"duration": 42.5,
			"created_at": "2026-01-15T10:00:00.000Z",
			"started_at": "2026-01-15T10:01:00.000Z",
			"finished_at": "2026-01-15T10:01:42.000Z"
		}
	]`

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/pipelines/1/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jobsJSON))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()

	result, err := provider.ListPipelineJobs(
		context.Background(),
		"owner/repo",
		1,
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
	)

	require.NoError(t, err)
	require.Len(t, result, 1)

	j := result[0]
	assert.Equal(t, "5", j.Id)
	assert.Equal(t, "build", j.Name)
	assert.Equal(t, "build", j.Stage)
	assert.Equal(t, "success", j.Status)
	require.NotNil(t, j.Ref)
	assert.Equal(t, "main", *j.Ref)
	require.NotNil(t, j.WebUrl)
	assert.Equal(t, "https://gitlab.com/project/-/jobs/5", *j.WebUrl)
	require.NotNil(t, j.AllowFailure)
	assert.False(t, *j.AllowFailure)
	require.NotNil(t, j.Duration)
	assert.InDelta(t, float32(42.5), *j.Duration, 0.01)
	assert.NotNil(t, j.CreatedAt)
	assert.NotNil(t, j.StartedAt)
	assert.NotNil(t, j.FinishedAt)
}

func TestGitLabProviderListPipelineJobsNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/pipelines/999/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Pipeline Not Found"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()

	result, err := provider.ListPipelineJobs(
		context.Background(),
		"owner/repo",
		999,
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, gferrors.ErrNotFound), "expected ErrNotFound, got: %v", err)
}

// --- GetJobTrace tests ---

func TestGitLabProviderGetJobTrace(t *testing.T) {
	traceContent := "Running with gitlab-runner...\nJob succeeded\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/jobs/7/trace", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(traceContent))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()

	result, truncated, err := provider.GetJobTrace(
		context.Background(),
		"owner/repo",
		7,
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
	)

	require.NoError(t, err)
	assert.Equal(t, traceContent, result)
	assert.False(t, truncated)
}

func TestGitLabProviderGetJobTraceNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/jobs/404/trace", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Job Not Found"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()

	result, truncated, err := provider.GetJobTrace(
		context.Background(),
		"owner/repo",
		404,
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
	)

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.False(t, truncated)
	assert.True(t, errors.Is(err, gferrors.ErrNotFound), "expected ErrNotFound, got: %v", err)
}

func TestGitLabProviderGetJobTraceSetsPrivateTokenHeader(t *testing.T) {
	var gotToken string

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/jobs/7/trace", func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("PRIVATE-TOKEN")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("log"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()

	_, _, err := provider.GetJobTrace(
		context.Background(),
		"owner/repo",
		7,
		krci.GitServerSettings{Token: "secret-token", Url: server.URL},
	)

	require.NoError(t, err)
	assert.Equal(t, "secret-token", gotToken)
}

func TestGitLabProviderGetJobTraceTruncated(t *testing.T) {
	// Serve a log larger than the cap to exercise the OOM guard: the result must be
	// exactly maxTraceBytes and flagged truncated.
	largeTrace := strings.Repeat("a", maxTraceBytes+1024)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/jobs/7/trace", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(largeTrace))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()

	result, truncated, err := provider.GetJobTrace(
		context.Background(),
		"owner/repo",
		7,
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
	)

	require.NoError(t, err)
	assert.True(t, truncated)
	assert.Len(t, result, maxTraceBytes)
}

func TestGitLabProviderGetJobTraceUnauthorized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/jobs/7/trace", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"401 Unauthorized"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()

	result, truncated, err := provider.GetJobTrace(
		context.Background(),
		"owner/repo",
		7,
		krci.GitServerSettings{Token: "bad-token", Url: server.URL},
	)

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.False(t, truncated)
	assert.True(t, errors.Is(err, gferrors.ErrUnauthorized), "expected ErrUnauthorized, got: %v", err)
}
