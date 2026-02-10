package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapPullRequestStateToGitLab(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  string
	}{
		{
			name:  "open maps to opened",
			state: "open",
			want:  "opened",
		},
		{
			name:  "closed maps to closed",
			state: "closed",
			want:  "closed",
		},
		{
			name:  "merged maps to merged",
			state: "merged",
			want:  "merged",
		},
		{
			name:  "all maps to all",
			state: "all",
			want:  "all",
		},
		{
			name:  "unknown state defaults to opened",
			state: "unknown",
			want:  "opened",
		},
		{
			name:  "empty state defaults to opened",
			state: "",
			want:  "opened",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPullRequestStateToGitLab(tt.state)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeGitLabMRState(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  models.PullRequestState
	}{
		{
			name:  "opened maps to open",
			state: "opened",
			want:  models.PullRequestStateOpen,
		},
		{
			name:  "merged maps to merged",
			state: "merged",
			want:  models.PullRequestStateMerged,
		},
		{
			name:  "closed maps to closed",
			state: "closed",
			want:  models.PullRequestStateClosed,
		},
		{
			name:  "locked defaults to open",
			state: "locked",
			want:  models.PullRequestStateOpen,
		},
		{
			name:  "unknown defaults to open",
			state: "unknown",
			want:  models.PullRequestStateOpen,
		},
		{
			name:  "empty defaults to open",
			state: "",
			want:  models.PullRequestStateOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitLabMRState(tt.state)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGitLabProviderListPullRequestsFieldMapping(t *testing.T) {
	createdAt := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 16, 14, 0, 0, 0, time.UTC)

	mrJSON := `[
		{
			"id": 1,
			"iid": 42,
			"title": "Amazing feature",
			"description": "This adds an amazing feature",
			"state": "opened",
			"draft": true,
			"sha": "abc123def456",
			"source_branch": "feature/amazing",
			"target_branch": "main",
			"web_url": "https://gitlab.com/owner/repo/-/merge_requests/42",
			"author": {
				"id": 999,
				"username": "johndoe",
				"avatar_url": "https://gitlab.com/avatar.png"
			},
			"created_at": "2026-01-15T10:30:00.000Z",
			"updated_at": "2026-01-16T14:00:00.000Z"
		}
	]`

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mrJSON))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()
	page := 1
	perPage := 20

	result, err := provider.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
		models.PullRequestListOptions{State: "open", Page: page, PerPage: perPage},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)

	pr := result.Data[0]
	assert.Equal(t, "1", pr.Id)
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "Amazing feature", pr.Title)
	assert.Equal(t, models.PullRequestStateOpen, pr.State)
	assert.Equal(t, "feature/amazing", pr.SourceBranch)
	assert.Equal(t, "main", pr.TargetBranch)
	assert.Equal(t, "https://gitlab.com/owner/repo/-/merge_requests/42", pr.Url)
	assert.Equal(t, createdAt, pr.CreatedAt)
	assert.Equal(t, updatedAt, pr.UpdatedAt)

	// Author mapping
	require.NotNil(t, pr.Author)
	assert.Equal(t, "999", pr.Author.Id)
	assert.Equal(t, "johndoe", pr.Author.Name)
	require.NotNil(t, pr.Author.AvatarUrl)
	assert.Equal(t, "https://gitlab.com/avatar.png", *pr.Author.AvatarUrl)

	// New fields: Description, Draft, CommitSha
	require.NotNil(t, pr.Description)
	assert.Equal(t, "This adds an amazing feature", *pr.Description)
	require.NotNil(t, pr.Draft)
	assert.True(t, *pr.Draft)
	require.NotNil(t, pr.CommitSha)
	assert.Equal(t, "abc123def456", *pr.CommitSha)

	// Pagination
	assert.Equal(t, 1, result.Pagination.Total)
	require.NotNil(t, result.Pagination.Page)
	assert.Equal(t, page, *result.Pagination.Page)
	require.NotNil(t, result.Pagination.PerPage)
	assert.Equal(t, perPage, *result.Pagination.PerPage)
}

func TestGitLabProviderListPullRequestsEmptyOptionalFields(t *testing.T) {
	// description is "", sha is "", draft is false.
	// Expect Description=nil, CommitSha=nil, Draft=&false.
	type mrResponse struct {
		ID           int    `json:"id"`
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		State        string `json:"state"`
		Draft        bool   `json:"draft"`
		SHA          string `json:"sha"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		WebURL       string `json:"web_url"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}

	mrs := []mrResponse{
		{
			ID:           2,
			IID:          10,
			Title:        "Simple change",
			Description:  "",
			State:        "opened",
			Draft:        false,
			SHA:          "",
			SourceBranch: "fix/typo",
			TargetBranch: "main",
			WebURL:       "https://gitlab.com/owner/repo/-/merge_requests/10",
			CreatedAt:    "2026-01-15T10:00:00.000Z",
			UpdatedAt:    "2026-01-15T11:00:00.000Z",
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total", "1")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mrs)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()

	result, err := provider.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
		models.PullRequestListOptions{State: "open", Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)

	pr := result.Data[0]

	assert.Nil(t, pr.Description, "description should be nil when empty string")
	assert.Nil(t, pr.CommitSha, "commit_sha should be nil when sha is empty string")
	require.NotNil(t, pr.Draft, "draft should always be present")
	assert.False(t, *pr.Draft)
}

func TestGitLabProviderListPullRequestsNilAuthor(t *testing.T) {
	mrJSON := `[
		{
			"id": 3,
			"iid": 15,
			"title": "MR without author",
			"description": "",
			"state": "opened",
			"draft": false,
			"sha": "",
			"source_branch": "orphan-branch",
			"target_branch": "main",
			"web_url": "https://gitlab.com/owner/repo/-/merge_requests/15",
			"author": null,
			"created_at": "2026-01-20T08:00:00.000Z",
			"updated_at": "2026-01-20T09:00:00.000Z"
		}
	]`

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mrJSON))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewGitlabProvider()

	result, err := provider.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: "test-token", Url: server.URL},
		models.PullRequestListOptions{State: "open", Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Nil(t, result.Data[0].Author, "author should be nil when JSON author is null")
}
