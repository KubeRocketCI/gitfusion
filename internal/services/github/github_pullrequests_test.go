package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	"github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T {
	return &v
}

func newTimestamp(t time.Time) *github.Timestamp {
	return &github.Timestamp{Time: t}
}

func TestGitHubProviderListPullRequestsStateMapping(t *testing.T) {
	createdAt := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 16, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		requestState string
		ghPRs        []*github.PullRequest
		wantGHState  string
		wantCount    int
		wantStates   []models.PullRequestState
		lastPage     int
	}{
		{
			name:         "open state passes open to GitHub API",
			requestState: "open",
			ghPRs: []*github.PullRequest{
				{
					ID:        ptr(int64(1)),
					Number:    ptr(1),
					Title:     ptr("Open PR"),
					State:     ptr("open"),
					HTMLURL:   ptr("https://github.com/owner/repo/pull/1"),
					Head:      &github.PullRequestBranch{Ref: ptr("feature")},
					Base:      &github.PullRequestBranch{Ref: ptr("main")},
					CreatedAt: newTimestamp(createdAt),
					UpdatedAt: newTimestamp(updatedAt),
				},
			},
			wantGHState: "open",
			wantCount:   1,
			wantStates:  []models.PullRequestState{models.PullRequestStateOpen},
		},
		{
			name:         "closed state passes closed to GitHub API and filters out merged",
			requestState: "closed",
			ghPRs: []*github.PullRequest{
				{
					ID:        ptr(int64(2)),
					Number:    ptr(2),
					Title:     ptr("Closed PR"),
					State:     ptr("closed"),
					HTMLURL:   ptr("https://github.com/owner/repo/pull/2"),
					Head:      &github.PullRequestBranch{Ref: ptr("fix")},
					Base:      &github.PullRequestBranch{Ref: ptr("main")},
					CreatedAt: newTimestamp(createdAt),
					UpdatedAt: newTimestamp(updatedAt),
					MergedAt:  nil, // Not merged
				},
				{
					ID:        ptr(int64(3)),
					Number:    ptr(3),
					Title:     ptr("Merged PR (should be filtered)"),
					State:     ptr("closed"),
					HTMLURL:   ptr("https://github.com/owner/repo/pull/3"),
					Head:      &github.PullRequestBranch{Ref: ptr("merged-fix")},
					Base:      &github.PullRequestBranch{Ref: ptr("main")},
					CreatedAt: newTimestamp(createdAt),
					UpdatedAt: newTimestamp(updatedAt),
					MergedAt:  newTimestamp(updatedAt), // Merged - should be filtered out
				},
			},
			wantGHState: "closed",
			wantCount:   1,
			wantStates:  []models.PullRequestState{models.PullRequestStateClosed},
		},
		{
			name:         "merged state passes closed to GitHub API and keeps only merged",
			requestState: "merged",
			ghPRs: []*github.PullRequest{
				{
					ID:        ptr(int64(4)),
					Number:    ptr(4),
					Title:     ptr("Closed but not merged (should be filtered)"),
					State:     ptr("closed"),
					HTMLURL:   ptr("https://github.com/owner/repo/pull/4"),
					Head:      &github.PullRequestBranch{Ref: ptr("feature-a")},
					Base:      &github.PullRequestBranch{Ref: ptr("main")},
					CreatedAt: newTimestamp(createdAt),
					UpdatedAt: newTimestamp(updatedAt),
					MergedAt:  nil, // Not merged - should be filtered out
				},
				{
					ID:        ptr(int64(5)),
					Number:    ptr(5),
					Title:     ptr("Merged PR"),
					State:     ptr("closed"),
					HTMLURL:   ptr("https://github.com/owner/repo/pull/5"),
					Head:      &github.PullRequestBranch{Ref: ptr("feature-b")},
					Base:      &github.PullRequestBranch{Ref: ptr("main")},
					CreatedAt: newTimestamp(createdAt),
					UpdatedAt: newTimestamp(updatedAt),
					MergedAt:  newTimestamp(updatedAt), // Merged - should be kept
				},
			},
			wantGHState: "closed",
			wantCount:   1,
			wantStates:  []models.PullRequestState{models.PullRequestStateMerged},
		},
		{
			name:         "all state passes all to GitHub API",
			requestState: "all",
			ghPRs: []*github.PullRequest{
				{
					ID:        ptr(int64(6)),
					Number:    ptr(6),
					Title:     ptr("Open PR"),
					State:     ptr("open"),
					HTMLURL:   ptr("https://github.com/owner/repo/pull/6"),
					Head:      &github.PullRequestBranch{Ref: ptr("feat-1")},
					Base:      &github.PullRequestBranch{Ref: ptr("main")},
					CreatedAt: newTimestamp(createdAt),
					UpdatedAt: newTimestamp(updatedAt),
				},
				{
					ID:        ptr(int64(7)),
					Number:    ptr(7),
					Title:     ptr("Merged PR"),
					State:     ptr("closed"),
					HTMLURL:   ptr("https://github.com/owner/repo/pull/7"),
					Head:      &github.PullRequestBranch{Ref: ptr("feat-2")},
					Base:      &github.PullRequestBranch{Ref: ptr("main")},
					CreatedAt: newTimestamp(createdAt),
					UpdatedAt: newTimestamp(updatedAt),
					MergedAt:  newTimestamp(updatedAt),
				},
				{
					ID:        ptr(int64(8)),
					Number:    ptr(8),
					Title:     ptr("Closed PR"),
					State:     ptr("closed"),
					HTMLURL:   ptr("https://github.com/owner/repo/pull/8"),
					Head:      &github.PullRequestBranch{Ref: ptr("feat-3")},
					Base:      &github.PullRequestBranch{Ref: ptr("main")},
					CreatedAt: newTimestamp(createdAt),
					UpdatedAt: newTimestamp(updatedAt),
				},
			},
			wantGHState: "all",
			wantCount:   3,
			wantStates: []models.PullRequestState{
				models.PullRequestStateOpen,
				models.PullRequestStateMerged,
				models.PullRequestStateClosed,
			},
		},
		{
			name:         "unknown state defaults to all",
			requestState: "something",
			ghPRs:        []*github.PullRequest{},
			wantGHState:  "all",
			wantCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedState string

			mux := http.NewServeMux()
			mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
				capturedState = r.URL.Query().Get("state")

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(tt.ghPRs)
			})

			server := httptest.NewServer(mux)
			defer server.Close()

			provider := newTestProvider(server.URL)

			result, err := provider.ListPullRequests(
				context.Background(),
				"owner",
				"repo",
				krci.GitServerSettings{Token: "test-token"},
				models.PullRequestListOptions{
					State:   tt.requestState,
					Page:    1,
					PerPage: 20,
				},
			)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantCount, len(result.Data))
			assert.Equal(t, tt.wantGHState, capturedState)

			for i, wantState := range tt.wantStates {
				assert.Equal(t, wantState, result.Data[i].State, "state mismatch at index %d", i)
			}
		})
	}
}

// githubRedirectTransport redirects GitHub API requests to the test server.
type githubRedirectTransport struct {
	target  string
	wrapped http.RoundTripper
}

func (t *githubRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	targetURL, _ := url.Parse(t.target)
	req.URL.Scheme = targetURL.Scheme
	req.URL.Host = targetURL.Host

	return t.wrapped.RoundTrip(req)
}

// newTestProvider creates a GitHubProvider with an HTTP client that redirects
// all requests to the given test server URL. This avoids mutating
// http.DefaultTransport and is safe for parallel tests.
func newTestProvider(serverURL string) *GitHubProvider {
	return &GitHubProvider{
		httpClient: &http.Client{
			Transport: &githubRedirectTransport{
				target:  serverURL,
				wrapped: http.DefaultTransport,
			},
		},
	}
}

func TestMapPullRequestStateToGitHub(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  string
	}{
		{
			name:  "open maps to open",
			state: "open",
			want:  "open",
		},
		{
			name:  "closed maps to closed",
			state: "closed",
			want:  "closed",
		},
		{
			name:  "merged maps to closed",
			state: "merged",
			want:  "closed",
		},
		{
			name:  "all maps to all",
			state: "all",
			want:  "all",
		},
		{
			name:  "unknown state defaults to all",
			state: "unknown",
			want:  "all",
		},
		{
			name:  "empty state defaults to all",
			state: "",
			want:  "all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPullRequestStateToGitHub(tt.state)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchesPullRequestStateFilter(t *testing.T) {
	mergedAt := time.Date(2026, 1, 17, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		pr    *github.PullRequest
		state string
		want  bool
	}{
		{
			name:  "merged filter matches PR with MergedAt",
			pr:    &github.PullRequest{MergedAt: newTimestamp(mergedAt)},
			state: "merged",
			want:  true,
		},
		{
			name:  "merged filter rejects PR without MergedAt",
			pr:    &github.PullRequest{MergedAt: nil},
			state: "merged",
			want:  false,
		},
		{
			name:  "closed filter matches PR without MergedAt",
			pr:    &github.PullRequest{MergedAt: nil},
			state: "closed",
			want:  true,
		},
		{
			name:  "closed filter rejects PR with MergedAt",
			pr:    &github.PullRequest{MergedAt: newTimestamp(mergedAt)},
			state: "closed",
			want:  false,
		},
		{
			name:  "open filter matches any PR",
			pr:    &github.PullRequest{MergedAt: newTimestamp(mergedAt)},
			state: "open",
			want:  true,
		},
		{
			name:  "all filter matches any PR",
			pr:    &github.PullRequest{MergedAt: nil},
			state: "all",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPullRequestStateFilter(tt.pr, tt.state)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertGitHubPullRequest(t *testing.T) {
	createdAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC)
	mergedAt := time.Date(2026, 1, 17, 10, 0, 0, 0, time.UTC)
	avatarURL := "https://github.com/avatars/1"

	tests := []struct {
		name       string
		pr         *github.PullRequest
		wantState  models.PullRequestState
		wantAuthor bool
	}{
		{
			name: "open PR",
			pr: &github.PullRequest{
				ID:        ptr(int64(1)),
				Number:    ptr(1),
				Title:     ptr("Open PR"),
				State:     ptr("open"),
				HTMLURL:   ptr("https://github.com/o/r/pull/1"),
				Head:      &github.PullRequestBranch{Ref: ptr("feat")},
				Base:      &github.PullRequestBranch{Ref: ptr("main")},
				CreatedAt: newTimestamp(createdAt),
				UpdatedAt: newTimestamp(updatedAt),
			},
			wantState: models.PullRequestStateOpen,
		},
		{
			name: "merged PR",
			pr: &github.PullRequest{
				ID:        ptr(int64(2)),
				Number:    ptr(2),
				Title:     ptr("Merged PR"),
				State:     ptr("closed"),
				HTMLURL:   ptr("https://github.com/o/r/pull/2"),
				Head:      &github.PullRequestBranch{Ref: ptr("feat")},
				Base:      &github.PullRequestBranch{Ref: ptr("main")},
				MergedAt:  newTimestamp(mergedAt),
				CreatedAt: newTimestamp(createdAt),
				UpdatedAt: newTimestamp(updatedAt),
			},
			wantState: models.PullRequestStateMerged,
		},
		{
			name: "closed PR without merge",
			pr: &github.PullRequest{
				ID:        ptr(int64(3)),
				Number:    ptr(3),
				Title:     ptr("Closed PR"),
				State:     ptr("closed"),
				HTMLURL:   ptr("https://github.com/o/r/pull/3"),
				Head:      &github.PullRequestBranch{Ref: ptr("feat")},
				Base:      &github.PullRequestBranch{Ref: ptr("main")},
				CreatedAt: newTimestamp(createdAt),
				UpdatedAt: newTimestamp(updatedAt),
			},
			wantState: models.PullRequestStateClosed,
		},
		{
			name: "PR with author",
			pr: &github.PullRequest{
				ID:      ptr(int64(4)),
				Number:  ptr(4),
				Title:   ptr("PR"),
				State:   ptr("open"),
				HTMLURL: ptr("https://github.com/o/r/pull/4"),
				Head:    &github.PullRequestBranch{Ref: ptr("feat")},
				Base:    &github.PullRequestBranch{Ref: ptr("main")},
				User: &github.User{
					ID:        ptr(int64(99)),
					Login:     ptr("dev"),
					AvatarURL: &avatarURL,
				},
				CreatedAt: newTimestamp(createdAt),
				UpdatedAt: newTimestamp(updatedAt),
			},
			wantState:  models.PullRequestStateOpen,
			wantAuthor: true,
		},
		{
			name: "draft PR with description and commit SHA",
			pr: &github.PullRequest{
				ID:        ptr(int64(5)),
				Number:    ptr(5),
				Title:     ptr("Draft PR"),
				Body:      ptr("Work in progress"),
				Draft:     ptr(true),
				State:     ptr("open"),
				HTMLURL:   ptr("https://github.com/o/r/pull/5"),
				Head:      &github.PullRequestBranch{Ref: ptr("wip"), SHA: ptr("sha256abc")},
				Base:      &github.PullRequestBranch{Ref: ptr("main")},
				CreatedAt: newTimestamp(createdAt),
				UpdatedAt: newTimestamp(updatedAt),
			},
			wantState: models.PullRequestStateOpen,
		},
		{
			name: "PR with empty optional fields",
			pr: &github.PullRequest{
				ID:        ptr(int64(6)),
				Number:    ptr(6),
				Title:     ptr("Minimal PR"),
				Body:      ptr(""),
				Draft:     ptr(false),
				State:     ptr("open"),
				HTMLURL:   ptr("https://github.com/o/r/pull/6"),
				Head:      &github.PullRequestBranch{Ref: ptr("fix"), SHA: ptr("")},
				Base:      &github.PullRequestBranch{Ref: ptr("main")},
				CreatedAt: newTimestamp(createdAt),
				UpdatedAt: newTimestamp(updatedAt),
			},
			wantState: models.PullRequestStateOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertGitHubPullRequest(tt.pr)
			assert.Equal(t, tt.wantState, got.State)

			if tt.wantAuthor {
				require.NotNil(t, got.Author)
				assert.Equal(t, "dev", got.Author.Name)
			}

			// Verify new fields are passed through
			assert.Equal(t, tt.pr.Draft, got.Draft)

			if tt.pr.Head != nil && tt.pr.Head.GetSHA() != "" {
				assert.Equal(t, tt.pr.Head.SHA, got.CommitSha)
			} else if tt.pr.Head == nil {
				assert.Nil(t, got.CommitSha)
			} else {
				assert.Nil(t, got.CommitSha, "empty SHA should produce nil CommitSha")
			}

			if tt.pr.GetBody() != "" {
				assert.Equal(t, tt.pr.Body, got.Description)
			} else {
				assert.Nil(t, got.Description, "nil or empty body should produce nil Description")
			}
		})
	}
}

func TestGitHubProviderListPullRequestsFieldMapping(t *testing.T) {
	createdAt := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 16, 14, 0, 0, 0, time.UTC)
	avatarURL := "https://github.com/avatars/42"

	ghPRs := []*github.PullRequest{
		{
			ID:      ptr(int64(12345)),
			Number:  ptr(42),
			Title:   ptr("Add amazing feature"),
			Body:    ptr("This PR adds an amazing feature"),
			Draft:   ptr(true),
			State:   ptr("open"),
			HTMLURL: ptr("https://github.com/owner/repo/pull/42"),
			Head:    &github.PullRequestBranch{Ref: ptr("feature/amazing"), SHA: ptr("abc123def456")},
			Base:    &github.PullRequestBranch{Ref: ptr("main")},
			User: &github.User{
				ID:        ptr(int64(999)),
				Login:     ptr("johndoe"),
				AvatarURL: &avatarURL,
			},
			CreatedAt: newTimestamp(createdAt),
			UpdatedAt: newTimestamp(updatedAt),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ghPRs)
	}))
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: "test-token"},
		models.PullRequestListOptions{State: "open", Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)

	pr := result.Data[0]
	assert.Equal(t, strconv.FormatInt(12345, 10), pr.Id)
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "Add amazing feature", pr.Title)
	assert.Equal(t, models.PullRequestStateOpen, pr.State)
	assert.Equal(t, "feature/amazing", pr.SourceBranch)
	assert.Equal(t, "main", pr.TargetBranch)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", pr.Url)
	assert.Equal(t, createdAt, pr.CreatedAt)
	assert.Equal(t, updatedAt, pr.UpdatedAt)

	// Author mapping
	require.NotNil(t, pr.Author)
	assert.Equal(t, "999", pr.Author.Id)
	assert.Equal(t, "johndoe", pr.Author.Name)
	require.NotNil(t, pr.Author.AvatarUrl)
	assert.Equal(t, avatarURL, *pr.Author.AvatarUrl)

	// New fields
	require.NotNil(t, pr.Description)
	assert.Equal(t, "This PR adds an amazing feature", *pr.Description)
	require.NotNil(t, pr.Draft)
	assert.True(t, *pr.Draft)
	require.NotNil(t, pr.CommitSha)
	assert.Equal(t, "abc123def456", *pr.CommitSha)
}

func TestGitHubProviderListPullRequestsNilAuthor(t *testing.T) {
	createdAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC)

	ghPRs := []*github.PullRequest{
		{
			ID:        ptr(int64(1)),
			Number:    ptr(1),
			Title:     ptr("PR without author"),
			State:     ptr("open"),
			HTMLURL:   ptr("https://github.com/owner/repo/pull/1"),
			Head:      &github.PullRequestBranch{Ref: ptr("feature")},
			Base:      &github.PullRequestBranch{Ref: ptr("main")},
			User:      nil,
			CreatedAt: newTimestamp(createdAt),
			UpdatedAt: newTimestamp(updatedAt),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ghPRs)
	}))
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: "test-token"},
		models.PullRequestListOptions{State: "open", Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Nil(t, result.Data[0].Author, "author should be nil when User is nil")
}

func TestGitHubProviderListPullRequestsPagination(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		ghPRCount int
		page      int
		perPage   int
		lastPage  int
		wantTotal int
	}{
		{
			name:      "last page known from response",
			state:     "open",
			ghPRCount: 20,
			page:      1,
			perPage:   20,
			lastPage:  5,
			wantTotal: 100, // 5 * 20
		},
		{
			name:      "last page of results (less than perPage)",
			state:     "open",
			ghPRCount: 8,
			page:      3,
			perPage:   20,
			lastPage:  0,
			wantTotal: 48, // (3-1)*20 + 8
		},
		{
			name:      "exact perPage results with no lastPage",
			state:     "open",
			ghPRCount: 20,
			page:      2,
			perPage:   20,
			lastPage:  0,
			wantTotal: 40, // 2 * 20
		},
		{
			name:      "merged state returns exact total when all pages exhausted",
			state:     "merged",
			ghPRCount: 5,
			page:      1,
			perPage:   20,
			lastPage:  0,
			wantTotal: 5,
		},
		{
			name:      "closed state returns exact total when all pages exhausted",
			state:     "closed",
			ghPRCount: 5,
			page:      1,
			perPage:   20,
			lastPage:  0,
			wantTotal: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
			updatedAt := time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC)
			mergedAt := time.Date(2026, 1, 17, 10, 0, 0, 0, time.UTC)

			ghPRs := make([]*github.PullRequest, 0, tt.ghPRCount)

			for i := 0; i < tt.ghPRCount; i++ {
				pr := &github.PullRequest{
					ID:        ptr(int64(i + 1)),
					Number:    ptr(i + 1),
					Title:     ptr("PR " + strconv.Itoa(i+1)),
					State:     ptr("closed"),
					HTMLURL:   ptr("https://github.com/owner/repo/pull/" + strconv.Itoa(i+1)),
					Head:      &github.PullRequestBranch{Ref: ptr("feature-" + strconv.Itoa(i+1))},
					Base:      &github.PullRequestBranch{Ref: ptr("main")},
					CreatedAt: newTimestamp(createdAt),
					UpdatedAt: newTimestamp(updatedAt),
				}

				// For merged/closed state tests, mark all as merged so they pass the post-filter
				if tt.state == "merged" {
					pr.MergedAt = newTimestamp(mergedAt)
				}

				// For open state tests, mark them as open
				if tt.state == "open" {
					pr.State = ptr("open")
				}

				ghPRs = append(ghPRs, pr)
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				if tt.lastPage > 0 {
					// Simulate Link header for pagination
					linkURL := "https://api.github.com/repos/owner/repo/pulls?page=" + strconv.Itoa(tt.lastPage)
					w.Header().Set("Link", `<`+linkURL+`>; rel="last"`)
				}

				_ = json.NewEncoder(w).Encode(ghPRs)
			}))
			defer server.Close()

			provider := newTestProvider(server.URL)

			result, err := provider.ListPullRequests(
				context.Background(),
				"owner",
				"repo",
				krci.GitServerSettings{Token: "test-token"},
				models.PullRequestListOptions{
					State:   tt.state,
					Page:    tt.page,
					PerPage: tt.perPage,
				},
			)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantTotal, result.Pagination.Total)
			require.NotNil(t, result.Pagination.Page)
			assert.Equal(t, tt.page, *result.Pagination.Page)
			require.NotNil(t, result.Pagination.PerPage)
			assert.Equal(t, tt.perPage, *result.Pagination.PerPage)
		})
	}
}

func TestGitHubProviderListPullRequestsPostFiltering(t *testing.T) {
	createdAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC)
	mergedAt := time.Date(2026, 1, 17, 10, 0, 0, 0, time.UTC)

	// Mix of merged and non-merged closed PRs
	ghPRs := []*github.PullRequest{
		{
			ID:        ptr(int64(1)),
			Number:    ptr(1),
			Title:     ptr("Closed without merge"),
			State:     ptr("closed"),
			HTMLURL:   ptr("https://github.com/owner/repo/pull/1"),
			Head:      &github.PullRequestBranch{Ref: ptr("fix-1")},
			Base:      &github.PullRequestBranch{Ref: ptr("main")},
			CreatedAt: newTimestamp(createdAt),
			UpdatedAt: newTimestamp(updatedAt),
			MergedAt:  nil,
		},
		{
			ID:        ptr(int64(2)),
			Number:    ptr(2),
			Title:     ptr("Merged PR"),
			State:     ptr("closed"),
			HTMLURL:   ptr("https://github.com/owner/repo/pull/2"),
			Head:      &github.PullRequestBranch{Ref: ptr("fix-2")},
			Base:      &github.PullRequestBranch{Ref: ptr("main")},
			CreatedAt: newTimestamp(createdAt),
			UpdatedAt: newTimestamp(updatedAt),
			MergedAt:  newTimestamp(mergedAt),
		},
		{
			ID:        ptr(int64(3)),
			Number:    ptr(3),
			Title:     ptr("Another closed"),
			State:     ptr("closed"),
			HTMLURL:   ptr("https://github.com/owner/repo/pull/3"),
			Head:      &github.PullRequestBranch{Ref: ptr("fix-3")},
			Base:      &github.PullRequestBranch{Ref: ptr("main")},
			CreatedAt: newTimestamp(createdAt),
			UpdatedAt: newTimestamp(updatedAt),
			MergedAt:  nil,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ghPRs)
	}))
	defer server.Close()

	provider := newTestProvider(server.URL)

	t.Run("merged filter keeps only merged PRs", func(t *testing.T) {
		result, err := provider.ListPullRequests(
			context.Background(), "owner", "repo",
			krci.GitServerSettings{Token: "test-token"},
			models.PullRequestListOptions{State: "merged", Page: 1, PerPage: 20},
		)

		require.NoError(t, err)
		require.Len(t, result.Data, 1)
		assert.Equal(t, "Merged PR", result.Data[0].Title)
		assert.Equal(t, models.PullRequestStateMerged, result.Data[0].State)
	})

	t.Run("closed filter keeps only non-merged closed PRs", func(t *testing.T) {
		result, err := provider.ListPullRequests(
			context.Background(), "owner", "repo",
			krci.GitServerSettings{Token: "test-token"},
			models.PullRequestListOptions{State: "closed", Page: 1, PerPage: 20},
		)

		require.NoError(t, err)
		require.Len(t, result.Data, 2)
		assert.Equal(t, "Closed without merge", result.Data[0].Title)
		assert.Equal(t, models.PullRequestStateClosed, result.Data[0].State)
		assert.Equal(t, "Another closed", result.Data[1].Title)
		assert.Equal(t, models.PullRequestStateClosed, result.Data[1].State)
	})
}

func TestGitHubProviderListPullRequestsEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]*github.PullRequest{})
	}))
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: "test-token"},
		models.PullRequestListOptions{State: "open", Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Data)
	assert.Equal(t, 0, result.Pagination.Total)
}

func TestGitHubProviderListPullRequestsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "internal error",
		})
	}))
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: "test-token"},
		models.PullRequestListOptions{State: "open", Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list pull requests")
}

// newClosedPR creates a closed (not merged) PR with the given ID.
func newClosedPR(id int, ts time.Time) *github.PullRequest {
	return &github.PullRequest{
		ID:        ptr(int64(id)),
		Number:    ptr(id),
		Title:     ptr("Closed PR " + strconv.Itoa(id)),
		State:     ptr("closed"),
		HTMLURL:   ptr("https://github.com/owner/repo/pull/" + strconv.Itoa(id)),
		Head:      &github.PullRequestBranch{Ref: ptr("branch-" + strconv.Itoa(id))},
		Base:      &github.PullRequestBranch{Ref: ptr("main")},
		CreatedAt: newTimestamp(ts),
		UpdatedAt: newTimestamp(ts),
		MergedAt:  nil,
	}
}

// newMergedPR creates a merged PR with the given ID.
func newMergedPR(id int, ts time.Time) *github.PullRequest {
	pr := newClosedPR(id, ts)
	pr.Title = ptr("Merged PR " + strconv.Itoa(id))
	pr.MergedAt = newTimestamp(ts)

	return pr
}

func TestGitHubProviderListPullRequestsMultiPageAccumulation(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	// Page 1: 3 closed (not merged), 2 merged = 5 items
	// Page 2: 1 closed, 4 merged = 5 items
	// Requesting state=merged, PerPage=5 should collect from both pages.
	page1 := []*github.PullRequest{
		newClosedPR(1, ts),
		newClosedPR(2, ts),
		newMergedPR(3, ts),
		newClosedPR(4, ts),
		newMergedPR(5, ts),
	}
	page2 := []*github.PullRequest{
		newClosedPR(6, ts),
		newMergedPR(7, ts),
		newMergedPR(8, ts),
		newMergedPR(9, ts),
		newMergedPR(10, ts),
	}

	var requestCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		pageParam := r.URL.Query().Get("page")

		w.Header().Set("Content-Type", "application/json")

		switch pageParam {
		case "", "1":
			// Link header pointing to page 2
			w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/pulls?page=2>; rel="next"`)
			_ = json.NewEncoder(w).Encode(page1)
		case "2":
			// No next page
			_ = json.NewEncoder(w).Encode(page2)
		default:
			_ = json.NewEncoder(w).Encode([]*github.PullRequest{})
		}
	}))
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPullRequests(
		context.Background(), "owner", "repo",
		krci.GitServerSettings{Token: "test-token"},
		models.PullRequestListOptions{State: "merged", Page: 1, PerPage: 5},
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have collected merged PRs from both pages: 3, 5, 7, 8, 9
	assert.Len(t, result.Data, 5)
	assert.Equal(t, 3, result.Data[0].Number)
	assert.Equal(t, 5, result.Data[1].Number)
	assert.Equal(t, 7, result.Data[2].Number)
	assert.Equal(t, 8, result.Data[3].Number)
	assert.Equal(t, 9, result.Data[4].Number)

	for _, pr := range result.Data {
		assert.Equal(t, models.PullRequestStateMerged, pr.State)
	}

	// Should have fetched 2 pages from GitHub
	assert.Equal(t, 2, requestCount)
}

func TestGitHubProviderListPullRequestsPostFilterPageSkip(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	// 8 merged PRs spread across pages, requesting Page=2, PerPage=3
	// Should skip first 3 merged and return the next 3.
	page1 := []*github.PullRequest{
		newClosedPR(1, ts),
		newMergedPR(2, ts), // merged #1 (skip)
		newMergedPR(3, ts), // merged #2 (skip)
		newClosedPR(4, ts),
		newMergedPR(5, ts), // merged #3 (skip)
	}
	page2 := []*github.PullRequest{
		newMergedPR(6, ts), // merged #4 (take)
		newClosedPR(7, ts),
		newMergedPR(8, ts),  // merged #5 (take)
		newMergedPR(9, ts),  // merged #6 (take)
		newMergedPR(10, ts), // merged #7 (not needed)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageParam := r.URL.Query().Get("page")

		w.Header().Set("Content-Type", "application/json")

		switch pageParam {
		case "", "1":
			w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/pulls?page=2>; rel="next"`)
			_ = json.NewEncoder(w).Encode(page1)
		case "2":
			_ = json.NewEncoder(w).Encode(page2)
		default:
			_ = json.NewEncoder(w).Encode([]*github.PullRequest{})
		}
	}))
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPullRequests(
		context.Background(), "owner", "repo",
		krci.GitServerSettings{Token: "test-token"},
		models.PullRequestListOptions{State: "merged", Page: 2, PerPage: 3},
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have skipped merged PRs 2,3,5 and returned 6,8,9
	require.Len(t, result.Data, 3)
	assert.Equal(t, 6, result.Data[0].Number)
	assert.Equal(t, 8, result.Data[1].Number)
	assert.Equal(t, 9, result.Data[2].Number)

	// Page and PerPage should be preserved
	require.NotNil(t, result.Pagination.Page)
	assert.Equal(t, 2, *result.Pagination.Page)
	require.NotNil(t, result.Pagination.PerPage)
	assert.Equal(t, 3, *result.Pagination.PerPage)
}

func TestGitHubProviderListPullRequestsEarlyReturnTotal(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	// All 5 are merged. Request PerPage=3.
	// Should return 3 items with total indicating more may exist.
	page1 := []*github.PullRequest{
		newMergedPR(1, ts),
		newMergedPR(2, ts),
		newMergedPR(3, ts),
		newMergedPR(4, ts),
		newMergedPR(5, ts),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/pulls?page=2>; rel="next"`)
		_ = json.NewEncoder(w).Encode(page1)
	}))
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPullRequests(
		context.Background(), "owner", "repo",
		krci.GitServerSettings{Token: "test-token"},
		models.PullRequestListOptions{State: "merged", Page: 1, PerPage: 3},
	)

	require.NoError(t, err)
	require.Len(t, result.Data, 3)

	// Total should be a lower-bound estimate: Page*PerPage + 1 = 4
	assert.Equal(t, 4, result.Pagination.Total)
}

func TestGitHubProviderListPullRequestsClosedFilterWithMixedPRs(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	// Mix of merged and truly-closed across two pages.
	// state=closed should return only non-merged ones.
	page1 := []*github.PullRequest{
		newMergedPR(1, ts), // filtered out
		newClosedPR(2, ts), // kept
		newMergedPR(3, ts), // filtered out
		newClosedPR(4, ts), // kept
		newMergedPR(5, ts), // filtered out
	}
	page2 := []*github.PullRequest{
		newClosedPR(6, ts), // kept
		newClosedPR(7, ts), // kept
		newMergedPR(8, ts), // filtered out
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageParam := r.URL.Query().Get("page")

		w.Header().Set("Content-Type", "application/json")

		switch pageParam {
		case "", "1":
			w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/pulls?page=2>; rel="next"`)
			_ = json.NewEncoder(w).Encode(page1)
		case "2":
			_ = json.NewEncoder(w).Encode(page2)
		default:
			_ = json.NewEncoder(w).Encode([]*github.PullRequest{})
		}
	}))
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPullRequests(
		context.Background(), "owner", "repo",
		krci.GitServerSettings{Token: "test-token"},
		models.PullRequestListOptions{State: "closed", Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should contain only non-merged closed PRs: 2, 4, 6, 7
	require.Len(t, result.Data, 4)
	assert.Equal(t, 2, result.Data[0].Number)
	assert.Equal(t, 4, result.Data[1].Number)
	assert.Equal(t, 6, result.Data[2].Number)
	assert.Equal(t, 7, result.Data[3].Number)

	for _, pr := range result.Data {
		assert.Equal(t, models.PullRequestStateClosed, pr.State)
	}

	// All pages exhausted, exact total
	assert.Equal(t, 4, result.Pagination.Total)
}

func TestGitHubProviderListPullRequestsAPIErrorOnSecondPage(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	page1 := []*github.PullRequest{
		newMergedPR(1, ts),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageParam := r.URL.Query().Get("page")

		w.Header().Set("Content-Type", "application/json")

		switch pageParam {
		case "", "1":
			w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/pulls?page=2>; rel="next"`)
			_ = json.NewEncoder(w).Encode(page1)
		case "2":
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "rate limited"})
		}
	}))
	defer server.Close()

	provider := newTestProvider(server.URL)

	result, err := provider.ListPullRequests(
		context.Background(), "owner", "repo",
		krci.GitServerSettings{Token: "test-token"},
		models.PullRequestListOptions{State: "merged", Page: 1, PerPage: 5},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list pull requests")
}

func TestGitHubProviderListPullRequestsContextCancellation(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	page1 := []*github.PullRequest{
		newMergedPR(1, ts),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/pulls?page=2>; rel="next"`)
		_ = json.NewEncoder(w).Encode(page1)
	}))
	defer server.Close()

	provider := newTestProvider(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := provider.ListPullRequests(
		ctx, "owner", "repo",
		krci.GitServerSettings{Token: "test-token"},
		models.PullRequestListOptions{State: "merged", Page: 1, PerPage: 5},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
}
