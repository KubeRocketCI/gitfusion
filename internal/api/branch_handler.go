package api

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/branches"
)

// BranchHandler handles requests related to branches (all providers).
type BranchHandler struct {
	branchesService *branches.BranchesService
}

// NewBranchHandler creates a new BranchHandler.
func NewBranchHandler(branchesService *branches.BranchesService) *BranchHandler {
	return &BranchHandler{
		branchesService: branchesService,
	}
}

// ListBranches implements api.StrictServerInterface.
func (h *BranchHandler) ListBranches(
	ctx context.Context,
	request ListBranchesRequestObject,
) (ListBranchesResponseObject, error) {
	branches, err := h.branchesService.ListBranches(
		ctx,
		request.Params.GitServer,
		request.Params.Owner,
		request.Params.RepoName,
		models.ListOptions{},
	)
	if err != nil {
		return nil, err
	}

	return ListBranches200JSONResponse{
		Data: branches,
	}, nil
}
