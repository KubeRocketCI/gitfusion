package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
)

// pipelineService abstracts the pipeline capabilities
// so the handler can be tested without a real service.
type pipelineService interface {
	TriggerPipeline(
		ctx context.Context,
		gitServerName, project, ref string,
		variables []models.PipelineVariable,
	) (*models.PipelineResponse, error)
	ListPipelines(
		ctx context.Context,
		gitServerName, project string,
		opts models.PipelineListOptions,
	) (*models.PipelinesResponse, error)
}

// PipelineHandler handles requests related to CI/CD pipelines (all providers).
type PipelineHandler struct {
	pipelinesService pipelineService
}

// NewPipelineHandler creates a new PipelineHandler.
func NewPipelineHandler(pipelinesService pipelineService) *PipelineHandler {
	return &PipelineHandler{
		pipelinesService: pipelinesService,
	}
}

// TriggerPipeline implements api.StrictServerInterface.
func (h *PipelineHandler) TriggerPipeline(
	ctx context.Context,
	request TriggerPipelineRequestObject,
) (TriggerPipelineResponseObject, error) {
	// Validate required parameters
	if request.Params.GitServer == "" {
		return TriggerPipeline400JSONResponse{
			Code:    fmt.Sprintf("%d", http.StatusBadRequest),
			Message: "gitServer parameter is required",
		}, nil
	}

	if request.Params.Project == "" {
		return TriggerPipeline400JSONResponse{
			Code:    fmt.Sprintf("%d", http.StatusBadRequest),
			Message: "project parameter is required",
		}, nil
	}

	if request.Params.Ref == "" {
		return TriggerPipeline400JSONResponse{
			Code:    fmt.Sprintf("%d", http.StatusBadRequest),
			Message: "ref parameter is required",
		}, nil
	}

	// Parse variables
	var variables []models.PipelineVariable
	if request.Params.Variables != nil && *request.Params.Variables != "" {
		if err := json.Unmarshal([]byte(*request.Params.Variables), &variables); err != nil {
			return TriggerPipeline400JSONResponse{
				Code:    fmt.Sprintf("%d", http.StatusBadRequest),
				Message: fmt.Sprintf("invalid variables JSON format (expected array of {key, value, variableType}): %v", err),
			}, nil
		}
	}

	// Call service
	pipeline, err := h.pipelinesService.TriggerPipeline(
		ctx,
		request.Params.GitServer,
		request.Params.Project,
		request.Params.Ref,
		variables,
	)
	if err != nil {
		return h.triggerErrResponse(err), nil
	}

	return TriggerPipeline201JSONResponse(*pipeline), nil
}

// ListPipelines implements api.StrictServerInterface.
func (h *PipelineHandler) ListPipelines(
	ctx context.Context,
	request ListPipelinesRequestObject,
) (ListPipelinesResponseObject, error) {
	page, perPage := clampPagination(request.Params.Page, request.Params.PerPage)

	var ref *string
	if request.Params.Ref != nil && *request.Params.Ref != "" {
		ref = request.Params.Ref
	}

	var status *string

	if request.Params.Status != nil {
		s := string(*request.Params.Status)
		status = &s
	}

	resp, err := h.pipelinesService.ListPipelines(
		ctx,
		request.Params.GitServer,
		request.Params.Project,
		models.PipelineListOptions{
			Ref:     ref,
			Status:  status,
			Page:    page,
			PerPage: perPage,
		},
	)
	if err != nil {
		return h.listErrResponse(err), nil
	}

	return ListPipelines200JSONResponse(*resp), nil
}

// triggerErrResponse maps errors to appropriate HTTP response objects for TriggerPipeline.
// This method must only be called when err is not nil.
func (h *PipelineHandler) triggerErrResponse(err error) TriggerPipelineResponseObject {
	if errors.Is(err, gferrors.ErrUnauthorized) {
		return TriggerPipeline401JSONResponse{
			Code:    fmt.Sprintf("%d", http.StatusUnauthorized),
			Message: err.Error(),
		}
	}

	if errors.Is(err, gferrors.ErrBadRequest) {
		return TriggerPipeline400JSONResponse{
			Code:    fmt.Sprintf("%d", http.StatusBadRequest),
			Message: err.Error(),
		}
	}

	if errors.Is(err, gferrors.ErrNotFound) {
		return TriggerPipeline404JSONResponse{
			Code:    fmt.Sprintf("%d", http.StatusNotFound),
			Message: err.Error(),
		}
	}

	return TriggerPipeline500JSONResponse{
		Code:    fmt.Sprintf("%d", http.StatusInternalServerError),
		Message: err.Error(),
	}
}

// listErrResponse maps errors to appropriate HTTP response objects for ListPipelines.
// This method must only be called when err is not nil.
func (h *PipelineHandler) listErrResponse(err error) ListPipelinesResponseObject {
	if errors.Is(err, gferrors.ErrUnauthorized) {
		return ListPipelines401JSONResponse{
			Code:    fmt.Sprintf("%d", http.StatusUnauthorized),
			Message: err.Error(),
		}
	}

	if errors.Is(err, gferrors.ErrBadRequest) {
		return ListPipelines400JSONResponse{
			Code:    fmt.Sprintf("%d", http.StatusBadRequest),
			Message: err.Error(),
		}
	}

	if errors.Is(err, gferrors.ErrNotFound) {
		return ListPipelines404JSONResponse{
			Code:    fmt.Sprintf("%d", http.StatusNotFound),
			Message: err.Error(),
		}
	}

	return ListPipelines500JSONResponse{
		Code:    fmt.Sprintf("%d", http.StatusInternalServerError),
		Message: err.Error(),
	}
}
