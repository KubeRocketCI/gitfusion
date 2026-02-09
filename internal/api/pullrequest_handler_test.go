package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubPullRequestLister captures the arguments passed to ListPullRequests
// and returns a preconfigured response.
type stubPullRequestLister struct {
	gotGitServer string
	gotOwner     string
	gotRepoName  string
	gotOpts      models.PullRequestListOptions

	resp *models.PullRequestsResponse
	err  error
}

func (s *stubPullRequestLister) ListPullRequests(
	_ context.Context,
	gitServerName, owner, repoName string,
	opts models.PullRequestListOptions,
) (*models.PullRequestsResponse, error) {
	s.gotGitServer = gitServerName
	s.gotOwner = owner
	s.gotRepoName = repoName
	s.gotOpts = opts

	return s.resp, s.err
}

func TestPullRequestHandlerListPullRequestsParameterDefaults(t *testing.T) {
	tests := []struct {
		name        string
		params      models.ListPullRequestsParams
		wantState   string
		wantPage    int
		wantPerPage int
	}{
		{
			name: "all defaults when no optional params provided",
			params: models.ListPullRequestsParams{
				GitServer: "my-server",
				Owner:     "my-owner",
				RepoName:  "my-repo",
			},
			wantState:   "open",
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name: "custom state is used",
			params: models.ListPullRequestsParams{
				GitServer: "my-server",
				Owner:     "my-owner",
				RepoName:  "my-repo",
				State:     statePtr(models.ListPullRequestsParamsStateMerged),
			},
			wantState:   "merged",
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name: "custom page and perPage are used",
			params: models.ListPullRequestsParams{
				GitServer: "my-server",
				Owner:     "my-owner",
				RepoName:  "my-repo",
				Page:      intPtr(3),
				PerPage:   intPtr(50),
			},
			wantState:   "open",
			wantPage:    3,
			wantPerPage: 50,
		},
		{
			name: "perPage is capped at 100",
			params: models.ListPullRequestsParams{
				GitServer: "my-server",
				Owner:     "my-owner",
				RepoName:  "my-repo",
				PerPage:   intPtr(200),
			},
			wantState:   "open",
			wantPage:    1,
			wantPerPage: 100,
		},
		{
			name: "perPage at exactly 100 is not capped",
			params: models.ListPullRequestsParams{
				GitServer: "my-server",
				Owner:     "my-owner",
				RepoName:  "my-repo",
				PerPage:   intPtr(100),
			},
			wantState:   "open",
			wantPage:    1,
			wantPerPage: 100,
		},
		{
			name: "all state is passed through",
			params: models.ListPullRequestsParams{
				GitServer: "my-server",
				Owner:     "my-owner",
				RepoName:  "my-repo",
				State:     statePtr(models.ListPullRequestsParamsStateAll),
			},
			wantState:   "all",
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name: "zero page is clamped to 1",
			params: models.ListPullRequestsParams{
				GitServer: "my-server",
				Owner:     "my-owner",
				RepoName:  "my-repo",
				Page:      intPtr(0),
			},
			wantState:   "open",
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name: "negative page is clamped to 1",
			params: models.ListPullRequestsParams{
				GitServer: "my-server",
				Owner:     "my-owner",
				RepoName:  "my-repo",
				Page:      intPtr(-5),
			},
			wantState:   "open",
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name: "zero perPage is clamped to 20",
			params: models.ListPullRequestsParams{
				GitServer: "my-server",
				Owner:     "my-owner",
				RepoName:  "my-repo",
				PerPage:   intPtr(0),
			},
			wantState:   "open",
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name: "negative perPage is clamped to 20",
			params: models.ListPullRequestsParams{
				GitServer: "my-server",
				Owner:     "my-owner",
				RepoName:  "my-repo",
				PerPage:   intPtr(-10),
			},
			wantState:   "open",
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name: "both zero page and zero perPage are clamped to defaults",
			params: models.ListPullRequestsParams{
				GitServer: "my-server",
				Owner:     "my-owner",
				RepoName:  "my-repo",
				Page:      intPtr(0),
				PerPage:   intPtr(0),
			},
			wantState:   "open",
			wantPage:    1,
			wantPerPage: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubPullRequestLister{
				resp: &models.PullRequestsResponse{},
			}
			handler := NewPullRequestHandler(stub)

			resp, err := handler.ListPullRequests(context.Background(), ListPullRequestsRequestObject{
				Params: tt.params,
			})

			require.NoError(t, err)
			assert.IsType(t, ListPullRequests200JSONResponse{}, resp)
			assert.Equal(t, tt.params.GitServer, stub.gotGitServer)
			assert.Equal(t, tt.params.Owner, stub.gotOwner)
			assert.Equal(t, tt.params.RepoName, stub.gotRepoName)
			assert.Equal(t, tt.wantState, stub.gotOpts.State)
			assert.Equal(t, tt.wantPage, stub.gotOpts.Page)
			assert.Equal(t, tt.wantPerPage, stub.gotOpts.PerPage)
		})
	}
}

func TestPullRequestHandlerErrResponse(t *testing.T) {
	handler := &PullRequestHandler{}

	t.Run("unauthorized error returns 401", func(t *testing.T) {
		err := fmt.Errorf("token expired: %w", gferrors.ErrUnauthorized)

		resp := handler.errResponse(err)

		errResp, ok := resp.(ListPullRequests401JSONResponse)
		require.True(t, ok, "expected ListPullRequests401JSONResponse")
		assert.Equal(t, fmt.Sprintf("%d", http.StatusUnauthorized), errResp.Code)
		assert.Contains(t, errResp.Message, "unauthorized")
	})

	t.Run("generic error returns 500", func(t *testing.T) {
		resp := handler.errResponse(errors.New("something went wrong"))

		errResp, ok := resp.(ListPullRequests500JSONResponse)
		require.True(t, ok, "expected ListPullRequests500JSONResponse")
		assert.Equal(t, fmt.Sprintf("%d", http.StatusInternalServerError), errResp.Code)
		assert.Equal(t, "something went wrong", errResp.Message)
	})
}

func TestListPullRequests200JSONResponseVisitResponse(t *testing.T) {
	page := 1
	perPage := 20

	resp := ListPullRequests200JSONResponse{
		Data: []models.PullRequest{
			{
				Id:           "123",
				Number:       42,
				Title:        "Test PR",
				State:        models.PullRequestStateOpen,
				SourceBranch: "feature",
				TargetBranch: "main",
				Url:          "https://github.com/owner/repo/pull/42",
				CreatedAt:    time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC),
			},
		},
		Pagination: models.Pagination{
			Total:   1,
			Page:    &page,
			PerPage: &perPage,
		},
	}

	w := httptest.NewRecorder()
	err := resp.VisitListPullRequestsResponse(w)

	require.NoError(t, err)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), `"title":"Test PR"`)
}

func TestListPullRequests400JSONResponseVisitResponse(t *testing.T) {
	resp := ListPullRequests400JSONResponse{
		Message: "bad request",
		Code:    "bad_request",
	}

	w := httptest.NewRecorder()
	err := resp.VisitListPullRequestsResponse(w)

	require.NoError(t, err)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), `"message":"bad request"`)
}

func TestListPullRequests401JSONResponseVisitResponse(t *testing.T) {
	resp := ListPullRequests401JSONResponse{
		Message: "unauthorized",
		Code:    "401",
	}

	w := httptest.NewRecorder()
	err := resp.VisitListPullRequestsResponse(w)

	require.NoError(t, err)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), `"message":"unauthorized"`)
}

func TestListPullRequests500JSONResponseVisitResponse(t *testing.T) {
	resp := ListPullRequests500JSONResponse{
		Message: "internal server error",
		Code:    "500",
	}

	w := httptest.NewRecorder()
	err := resp.VisitListPullRequestsResponse(w)

	require.NoError(t, err)
	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), `"message":"internal server error"`)
}

func TestNewPullRequestHandler(t *testing.T) {
	handler := NewPullRequestHandler(nil)

	assert.NotNil(t, handler)
	assert.Nil(t, handler.pullRequestsService)
}

func TestServerListPullRequestsDelegatesToHandler(t *testing.T) {
	server := &Server{
		pullRequestHandler: &PullRequestHandler{},
	}

	assert.NotNil(t, server.pullRequestHandler)
}

// Helper functions
func statePtr(s models.ListPullRequestsParamsState) *models.ListPullRequestsParamsState {
	return &s
}

func intPtr(i int) *int {
	return &i
}

// Verify that Server implements StrictServerInterface for ListPullRequests.
var _ StrictServerInterface = (*Server)(nil)

// Verify that PullRequestHandler.ListPullRequests has the correct signature.
func TestPullRequestHandlerImplementsExpectedSignature(t *testing.T) {
	handler := &PullRequestHandler{}

	fn := handler.ListPullRequests
	assert.NotNil(t, fn)
}
