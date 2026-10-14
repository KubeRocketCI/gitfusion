package api

import (
	"context"
	"encoding/json"
	"strconv"

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
	ListPipelineJobs(
		ctx context.Context,
		gitServerName, project string,
		pipelineID int,
	) ([]models.PipelineJob, error)
	GetJobTrace(
		ctx context.Context,
		gitServerName, project string,
		jobID int,
	) (content string, truncated bool, err error)
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
	var variables []models.PipelineVariable
	if request.Params.Variables != nil && *request.Params.Variables != "" {
		if err := json.Unmarshal([]byte(*request.Params.Variables), &variables); err != nil {
			return nil, &gferrors.InvalidParamError{Param: "variables", Err: err}
		}
	}

	pipeline, err := h.pipelinesService.TriggerPipeline(
		ctx,
		request.Params.GitServer,
		request.Params.Project,
		request.Params.Ref,
		variables,
	)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	return ListPipelines200JSONResponse(*resp), nil
}

// ListPipelineJobs implements api.StrictServerInterface.
func (h *PipelineHandler) ListPipelineJobs(
	ctx context.Context,
	request ListPipelineJobsRequestObject,
) (ListPipelineJobsResponseObject, error) {
	pipelineID, err := strconv.Atoi(request.Params.PipelineId)
	if err != nil {
		return nil, &gferrors.InvalidParamError{Param: "pipelineId", Err: err}
	}

	jobs, err := h.pipelinesService.ListPipelineJobs(ctx, request.Params.GitServer, request.Params.Project, pipelineID)
	if err != nil {
		return nil, err
	}

	return ListPipelineJobs200JSONResponse(models.PipelineJobsResponse{Data: jobs}), nil
}

// GetPipelineJobTrace implements api.StrictServerInterface.
func (h *PipelineHandler) GetPipelineJobTrace(
	ctx context.Context,
	request GetPipelineJobTraceRequestObject,
) (GetPipelineJobTraceResponseObject, error) {
	jobID, err := strconv.Atoi(request.Params.JobId)
	if err != nil {
		return nil, &gferrors.InvalidParamError{Param: "jobId", Err: err}
	}

	trace, truncated, err := h.pipelinesService.GetJobTrace(ctx, request.Params.GitServer, request.Params.Project, jobID)
	if err != nil {
		return nil, err
	}

	resp := models.PipelineJobTraceResponse{
		JobId:     request.Params.JobId,
		Content:   trace,
		Truncated: &truncated,
	}

	return GetPipelineJobTrace200JSONResponse(resp), nil
}
