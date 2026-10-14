package api

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/models"
)

// pullRequestLister abstracts the pull-request listing capability
// so the handler can be tested without a real service.
type pullRequestLister interface {
	ListPullRequests(
		ctx context.Context,
		gitServerName, owner, repoName string,
		opts models.PullRequestListOptions,
	) (*models.PullRequestsResponse, error)
}

// PullRequestHandler handles requests related to pull/merge requests (all providers).
type PullRequestHandler struct {
	pullRequestsService pullRequestLister
}

// NewPullRequestHandler creates a new PullRequestHandler.
func NewPullRequestHandler(pullRequestsService pullRequestLister) *PullRequestHandler {
	return &PullRequestHandler{
		pullRequestsService: pullRequestsService,
	}
}

// ListPullRequests implements api.StrictServerInterface.
func (h *PullRequestHandler) ListPullRequests(
	ctx context.Context,
	request ListPullRequestsRequestObject,
) (ListPullRequestsResponseObject, error) {
	// Apply defaults for optional parameters
	state := "open"
	if request.Params.State != nil {
		state = string(*request.Params.State)
	}

	page, perPage := clampPagination(request.Params.Page, request.Params.PerPage)

	resp, err := h.pullRequestsService.ListPullRequests(
		ctx,
		request.Params.GitServer,
		request.Params.Owner,
		request.Params.RepoName,
		models.PullRequestListOptions{
			State:   state,
			Page:    page,
			PerPage: perPage,
		},
	)
	if err != nil {
		return nil, err
	}

	return ListPullRequests200JSONResponse(*resp), nil
}
