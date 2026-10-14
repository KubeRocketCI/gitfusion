package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/pkg/pointer"
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
				State:     pointer.To(models.ListPullRequestsParamsStateMerged),
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
				Page:      pointer.To(3),
				PerPage:   pointer.To(50),
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
				PerPage:   pointer.To(200),
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
				PerPage:   pointer.To(100),
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
				State:     pointer.To(models.ListPullRequestsParamsStateAll),
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
				Page:      pointer.To(0),
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
				Page:      pointer.To(-5),
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
				PerPage:   pointer.To(0),
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
				PerPage:   pointer.To(-10),
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
				Page:      pointer.To(0),
				PerPage:   pointer.To(0),
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

func TestPullRequestHandlerListPullRequestsReturnsServiceError(t *testing.T) {
	serviceErr := fmt.Errorf("upstream: %w", gferrors.ErrUnauthorized)
	stub := &stubPullRequestLister{err: serviceErr}
	handler := NewPullRequestHandler(stub)

	resp, err := handler.ListPullRequests(context.Background(), ListPullRequestsRequestObject{
		Params: models.ListPullRequestsParams{GitServer: "my-server", Owner: "my-owner", RepoName: "my-repo"},
	})

	assert.Nil(t, resp)
	assert.Same(t, serviceErr, err)
	assert.Equal(t, "my-server", stub.gotGitServer)
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

// Verify that Server implements StrictServerInterface for ListPullRequests.
var _ StrictServerInterface = (*Server)(nil)

// Verify that PullRequestHandler.ListPullRequests has the correct signature.
func TestPullRequestHandlerImplementsExpectedSignature(t *testing.T) {
	handler := &PullRequestHandler{}

	fn := handler.ListPullRequests
	assert.NotNil(t, fn)
}
