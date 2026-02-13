package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

func TestNormalizeGitHubWorkflowRunStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		conclusion string
		want       models.PipelineStatus
	}{
		{name: "queued maps to pending", status: "queued", conclusion: "", want: models.PipelineStatusPending},
		{name: "pending maps to pending", status: "pending", conclusion: "", want: models.PipelineStatusPending},
		{name: "waiting maps to pending", status: "waiting", conclusion: "", want: models.PipelineStatusPending},
		{name: "requested maps to pending", status: "requested", conclusion: "", want: models.PipelineStatusPending},
		{name: "in_progress maps to running", status: "in_progress", conclusion: "", want: models.PipelineStatusRunning},
		{
			name: "completed+success maps to success", status: "completed",
			conclusion: "success", want: models.PipelineStatusSuccess,
		},
		{
			name: "completed+failure maps to failed", status: "completed",
			conclusion: "failure", want: models.PipelineStatusFailed,
		},
		{
			name: "completed+timed_out maps to failed", status: "completed",
			conclusion: "timed_out", want: models.PipelineStatusFailed,
		},
		{
			name: "completed+startup_failure maps to failed", status: "completed",
			conclusion: "startup_failure", want: models.PipelineStatusFailed,
		},
		{
			name: "completed+cancelled maps to cancelled", status: "completed",
			conclusion: "cancelled", want: models.PipelineStatusCancelled,
		},
		{
			name: "completed+skipped maps to skipped", status: "completed",
			conclusion: "skipped", want: models.PipelineStatusSkipped,
		},
		{
			name: "completed+action_required maps to manual", status: "completed",
			conclusion: "action_required", want: models.PipelineStatusManual,
		},
		{
			name: "completed+neutral maps to success", status: "completed",
			conclusion: "neutral", want: models.PipelineStatusSuccess,
		},
		{
			name: "completed+stale maps to cancelled", status: "completed",
			conclusion: "stale", want: models.PipelineStatusCancelled,
		},
		{
			name: "completed+unknown conclusion defaults to failed", status: "completed",
			conclusion: "something", want: models.PipelineStatusFailed,
		},
		{name: "unknown status defaults to pending", status: "something", conclusion: "", want: models.PipelineStatusPending},
		{name: "empty status defaults to pending", status: "", conclusion: "", want: models.PipelineStatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitHubWorkflowRunStatus(tt.status, tt.conclusion)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeGitHubWorkflowRunEvent(t *testing.T) {
	tests := []struct {
		name  string
		event string
		want  models.PipelineSource
	}{
		{name: "push", event: "push", want: models.PipelineSourcePush},
		{name: "pull_request maps to merge_request", event: "pull_request", want: models.PipelineSourceMergeRequest},
		{
			name:  "pull_request_target maps to merge_request",
			event: "pull_request_target", want: models.PipelineSourceMergeRequest,
		},
		{name: "schedule", event: "schedule", want: models.PipelineSourceSchedule},
		{name: "workflow_dispatch maps to manual", event: "workflow_dispatch", want: models.PipelineSourceManual},
		{name: "repository_dispatch maps to trigger", event: "repository_dispatch", want: models.PipelineSourceTrigger},
		{name: "workflow_call maps to trigger", event: "workflow_call", want: models.PipelineSourceTrigger},
		{name: "unknown maps to other", event: "unknown", want: models.PipelineSourceOther},
		{name: "empty maps to other", event: "", want: models.PipelineSourceOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitHubWorkflowRunEvent(tt.event)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapPipelineStatusToGitHub(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		wantNil bool
		wantVal string
	}{
		{name: "pending maps to queued", status: "pending", wantNil: false, wantVal: "queued"},
		{name: "running maps to in_progress", status: "running", wantNil: false, wantVal: "in_progress"},
		{name: "success maps to success", status: "success", wantNil: false, wantVal: "success"},
		{name: "failed maps to failure", status: "failed", wantNil: false, wantVal: "failure"},
		{name: "cancelled maps to cancelled", status: "cancelled", wantNil: false, wantVal: "cancelled"},
		{name: "skipped maps to skipped", status: "skipped", wantNil: false, wantVal: "skipped"},
		{name: "manual maps to action_required", status: "manual", wantNil: false, wantVal: "action_required"},
		{name: "unknown returns nil", status: "unknown", wantNil: true},
		{name: "empty returns nil", status: "", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPipelineStatusToGitHub(tt.status)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.wantVal, *got)
			}
		})
	}
}

// mustParseTime parses an RFC3339 time string and panics on failure.
func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}

	return t
}

func TestGitHubProviderListPipelinesFieldMapping(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/actions/runs", func(w http.ResponseWriter, r *http.Request) {
		resp := github.WorkflowRuns{
			TotalCount: ptr(1),
			WorkflowRuns: []*github.WorkflowRun{
				{
					ID:         ptr(int64(12345)),
					Status:     ptr("completed"),
					Conclusion: ptr("success"),
					HeadBranch: ptr("main"),
					HeadSHA:    ptr("abc123def456"),
					HTMLURL:    ptr("https://github.com/owner/repo/actions/runs/12345"),
					Event:      ptr("push"),
					CreatedAt:  newTimestamp(mustParseTime("2026-01-15T10:30:00Z")),
					UpdatedAt:  newTimestamp(mustParseTime("2026-01-16T14:00:00Z")),
					Repository: &github.Repository{
						ID: ptr(int64(42)),
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: "test-token"},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)

	p := result.Data[0]
	assert.Equal(t, "12345", p.Id)
	assert.Equal(t, models.PipelineStatusSuccess, p.Status)
	assert.Equal(t, "main", p.Ref)
	assert.Equal(t, "abc123def456", p.Sha)
	assert.Equal(t, "https://github.com/owner/repo/actions/runs/12345", p.WebUrl)

	// Source mapping
	require.NotNil(t, p.Source)
	assert.Equal(t, models.PipelineSourcePush, *p.Source)

	// ProjectId mapping
	require.NotNil(t, p.ProjectId)
	assert.Equal(t, "42", *p.ProjectId)

	// Timestamps
	assert.Equal(t, mustParseTime("2026-01-15T10:30:00Z"), p.CreatedAt)
	require.NotNil(t, p.UpdatedAt)
	assert.Equal(t, mustParseTime("2026-01-16T14:00:00Z"), *p.UpdatedAt)

	// Pagination
	assert.Equal(t, 1, result.Pagination.Total)
	require.NotNil(t, result.Pagination.Page)
	assert.Equal(t, 1, *result.Pagination.Page)
	require.NotNil(t, result.Pagination.PerPage)
	assert.Equal(t, 20, *result.Pagination.PerPage)
}

func TestGitHubProviderListPipelinesMultipleStatuses(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/actions/runs", func(w http.ResponseWriter, r *http.Request) {
		resp := github.WorkflowRuns{
			TotalCount: ptr(3),
			WorkflowRuns: []*github.WorkflowRun{
				{
					ID:         ptr(int64(1)),
					Status:     ptr("in_progress"),
					Conclusion: ptr(""),
					HeadBranch: ptr("main"),
					HeadSHA:    ptr("aaa"),
					HTMLURL:    ptr("https://github.com/o/r/actions/runs/1"),
					Event:      ptr("push"),
					CreatedAt:  newTimestamp(mustParseTime("2026-01-15T10:00:00Z")),
				},
				{
					ID:         ptr(int64(2)),
					Status:     ptr("completed"),
					Conclusion: ptr("failure"),
					HeadBranch: ptr("feature"),
					HeadSHA:    ptr("bbb"),
					HTMLURL:    ptr("https://github.com/o/r/actions/runs/2"),
					Event:      ptr("pull_request"),
					CreatedAt:  newTimestamp(mustParseTime("2026-01-15T11:00:00Z")),
				},
				{
					ID:         ptr(int64(3)),
					Status:     ptr("completed"),
					Conclusion: ptr("cancelled"),
					HeadBranch: ptr("main"),
					HeadSHA:    ptr("ccc"),
					HTMLURL:    ptr("https://github.com/o/r/actions/runs/3"),
					Event:      ptr("workflow_dispatch"),
					CreatedAt:  newTimestamp(mustParseTime("2026-01-15T12:00:00Z")),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: "test-token"},
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

func TestGitHubProviderListPipelinesFilterParams(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/actions/runs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "main", r.URL.Query().Get("branch"))
		assert.Equal(t, "success", r.URL.Query().Get("status"))

		resp := github.WorkflowRuns{
			TotalCount: ptr(1),
			WorkflowRuns: []*github.WorkflowRun{
				{
					ID:         ptr(int64(100)),
					Status:     ptr("completed"),
					Conclusion: ptr("success"),
					HeadBranch: ptr("main"),
					HeadSHA:    ptr("abc123"),
					HTMLURL:    ptr("https://github.com/owner/repo/actions/runs/100"),
					Event:      ptr("push"),
					CreatedAt:  newTimestamp(mustParseTime("2026-01-15T10:30:00Z")),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newTestProvider(server.URL)

	ref := "main"
	status := "success"

	result, err := provider.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: "test-token"},
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

func TestGitHubProviderListPipelinesNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/nonexistent/project/actions/runs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "Not Found",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPipelines(
		context.Background(),
		"nonexistent/project",
		krci.GitServerSettings{Token: "test-token"},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestGitHubProviderListPipelinesUnauthorized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/actions/runs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "Bad credentials",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: "bad-token"},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestGitHubProviderTriggerPipelineUnsupported(t *testing.T) {
	provider := NewGitHubProvider()

	result, err := provider.TriggerPipeline(
		context.Background(),
		"owner/repo",
		"main",
		nil,
		krci.GitServerSettings{Token: "test-token"},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not supported for GitHub")
}
