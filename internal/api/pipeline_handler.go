package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/pipelines"
)

// PipelineHandler handles requests related to CI/CD pipelines (all providers).
type PipelineHandler struct {
	pipelinesService *pipelines.PipelinesService
}

// NewPipelineHandler creates a new PipelineHandler.
func NewPipelineHandler(pipelinesService *pipelines.PipelinesService) *PipelineHandler {
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
		return h.errResponse(err), nil
	}

	return TriggerPipeline201JSONResponse(*pipeline), nil
}

// errResponse maps errors to appropriate HTTP response objects.
// This method must only be called when err is not nil.
func (h *PipelineHandler) errResponse(err error) TriggerPipelineResponseObject {
	if errors.Is(err, gferrors.ErrNotFound) {
		return TriggerPipeline404JSONResponse{
			Code:    fmt.Sprintf("%d", http.StatusNotFound),
			Message: err.Error(),
		}
	}

	if errors.Is(err, gferrors.ErrUnauthorized) {
		return TriggerPipeline401JSONResponse{
			Code:    fmt.Sprintf("%d", http.StatusUnauthorized),
			Message: err.Error(),
		}
	}

	return TriggerPipeline500JSONResponse{
		Code:    fmt.Sprintf("%d", http.StatusInternalServerError),
		Message: err.Error(),
	}
}
