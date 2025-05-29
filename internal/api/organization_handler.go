package api

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/services"
)

// OrganizationHandler handles requests related to organizations (all providers).
type OrganizationHandler struct {
	organizationsService *services.OrganizationsService
}

// NewOrganizationHandler creates a new OrganizationHandler.
func NewOrganizationHandler(organizationsService *services.OrganizationsService) *OrganizationHandler {
	return &OrganizationHandler{
		organizationsService: organizationsService,
	}
}

// ListUserOrganizations implements api.StrictServerInterface.
func (h *OrganizationHandler) ListUserOrganizations(
	ctx context.Context,
	request ListUserOrganizationsRequestObject,
) (ListUserOrganizationsResponseObject, error) {
	orgs, err := h.organizationsService.ListUserOrganizations(ctx, request.Params.GitServer)
	if err != nil {
		return ListUserOrganizations400JSONResponse{
			Message: err.Error(),
			Code:    "bad_request",
		}, nil
	}

	return ListUserOrganizations200JSONResponse{
		Data: orgs,
	}, nil
}
