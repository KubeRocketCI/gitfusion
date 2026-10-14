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

// stubPipelineService captures the arguments passed to TriggerPipeline and ListPipelines
// and returns preconfigured responses.
type stubPipelineService struct {
	// TriggerPipeline captures
	gotTriggerGitServer string
	gotTriggerProject   string
	gotTriggerRef       string
	gotTriggerVars      []models.PipelineVariable
	triggerResp         *models.PipelineResponse
	triggerErr          error

	// ListPipelines captures
	gotListGitServer string
	gotListProject   string
	gotListOpts      models.PipelineListOptions
	listResp         *models.PipelinesResponse
	listErr          error

	// ListPipelineJobs captures
	gotJobsGitServer  string
	gotJobsProject    string
	gotJobsPipelineID int
	jobsResp          []models.PipelineJob
	jobsErr           error

	// GetJobTrace captures
	gotTraceGitServer string
	gotTraceProject   string
	gotTraceJobID     int
	traceResp         string
	traceTruncated    bool
	traceErr          error
}

func (s *stubPipelineService) TriggerPipeline(
	_ context.Context,
	gitServerName, project, ref string,
	variables []models.PipelineVariable,
) (*models.PipelineResponse, error) {
	s.gotTriggerGitServer = gitServerName
	s.gotTriggerProject = project
	s.gotTriggerRef = ref
	s.gotTriggerVars = variables

	return s.triggerResp, s.triggerErr
}

func (s *stubPipelineService) ListPipelines(
	_ context.Context,
	gitServerName, project string,
	opts models.PipelineListOptions,
) (*models.PipelinesResponse, error) {
	s.gotListGitServer = gitServerName
	s.gotListProject = project
	s.gotListOpts = opts

	return s.listResp, s.listErr
}

func (s *stubPipelineService) ListPipelineJobs(
	_ context.Context,
	gitServerName, project string,
	pipelineID int,
) ([]models.PipelineJob, error) {
	s.gotJobsGitServer = gitServerName
	s.gotJobsProject = project
	s.gotJobsPipelineID = pipelineID

	return s.jobsResp, s.jobsErr
}

func (s *stubPipelineService) GetJobTrace(
	_ context.Context,
	gitServerName, project string,
	jobID int,
) (string, bool, error) {
	s.gotTraceGitServer = gitServerName
	s.gotTraceProject = project
	s.gotTraceJobID = jobID

	return s.traceResp, s.traceTruncated, s.traceErr
}

// --- TriggerPipeline tests ---

func TestPipelineHandlerTriggerPipelineInvalidVariables(t *testing.T) {
	stub := &stubPipelineService{}
	handler := NewPipelineHandler(stub)

	resp, err := handler.TriggerPipeline(context.Background(), TriggerPipelineRequestObject{
		Params: models.TriggerPipelineParams{
			GitServer: "my-server",
			Project:   "my-project",
			Ref:       "main",
			Variables: pointer.To("{not-json"),
		},
	})

	assert.Nil(t, resp)
	requireInvalidParam(t, err, "variables")
	assert.Empty(t, stub.gotTriggerGitServer, "service must not be called")
}

func TestPipelineHandlerTriggerPipelineSuccess(t *testing.T) {
	stub := &stubPipelineService{
		triggerResp: &models.PipelineResponse{
			Id:     1,
			WebUrl: "https://gitlab.com/project/-/pipelines/1",
			Status: "running",
			Ref:    "main",
		},
	}
	handler := NewPipelineHandler(stub)

	resp, err := handler.TriggerPipeline(context.Background(), TriggerPipelineRequestObject{
		Params: models.TriggerPipelineParams{
			GitServer: "my-server",
			Project:   "my-project",
			Ref:       "main",
		},
	})

	require.NoError(t, err)
	assert.IsType(t, TriggerPipeline201JSONResponse{}, resp)
	assert.Equal(t, "my-server", stub.gotTriggerGitServer)
	assert.Equal(t, "my-project", stub.gotTriggerProject)
	assert.Equal(t, "main", stub.gotTriggerRef)
}

func TestPipelineHandlerTriggerPipelineForwardsVariables(t *testing.T) {
	stub := &stubPipelineService{triggerResp: &models.PipelineResponse{}}
	handler := NewPipelineHandler(stub)

	resp, err := handler.TriggerPipeline(context.Background(), TriggerPipelineRequestObject{
		Params: models.TriggerPipelineParams{
			GitServer: "my-server",
			Project:   "my-project",
			Ref:       "main",
			Variables: pointer.To(`[{"key":"K","value":"V"}]`),
		},
	})

	require.NoError(t, err)
	assert.IsType(t, TriggerPipeline201JSONResponse{}, resp)
	assert.Equal(t, []models.PipelineVariable{{Key: "K", Value: "V"}}, stub.gotTriggerVars)
}

func TestPipelineHandlerReturnsServiceError(t *testing.T) {
	serviceErr := fmt.Errorf("upstream: %w", gferrors.ErrNotFound)
	stub := &stubPipelineService{
		triggerErr: serviceErr,
		listErr:    serviceErr,
		jobsErr:    serviceErr,
		traceErr:   serviceErr,
	}
	handler := NewPipelineHandler(stub)
	ctx := context.Background()

	tests := []struct {
		name string
		call func() (any, error)
	}{
		{
			name: "TriggerPipeline",
			call: func() (any, error) {
				return handler.TriggerPipeline(ctx, TriggerPipelineRequestObject{})
			},
		},
		{
			name: "ListPipelines",
			call: func() (any, error) {
				return handler.ListPipelines(ctx, ListPipelinesRequestObject{})
			},
		},
		{
			name: "ListPipelineJobs",
			call: func() (any, error) {
				return handler.ListPipelineJobs(ctx, ListPipelineJobsRequestObject{
					Params: models.ListPipelineJobsParams{PipelineId: "5"},
				})
			},
		},
		{
			name: "GetPipelineJobTrace",
			call: func() (any, error) {
				return handler.GetPipelineJobTrace(ctx, GetPipelineJobTraceRequestObject{
					Params: models.GetPipelineJobTraceParams{JobId: "23"},
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.call()

			assert.Nil(t, resp)
			assert.Same(t, serviceErr, err)
		})
	}
}

// --- ListPipelines tests ---

func TestPipelineHandlerListPipelinesParameterDefaults(t *testing.T) {
	tests := []struct {
		name        string
		params      models.ListPipelinesParams
		wantPage    int
		wantPerPage int
	}{
		{
			name: "all defaults when no optional params provided",
			params: models.ListPipelinesParams{
				GitServer: "my-server",
				Project:   "my-project",
			},
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name: "custom page and perPage are used",
			params: models.ListPipelinesParams{
				GitServer: "my-server",
				Project:   "my-project",
				Page:      pointer.To(3),
				PerPage:   pointer.To(50),
			},
			wantPage:    3,
			wantPerPage: 50,
		},
		{
			name: "perPage is capped at 100",
			params: models.ListPipelinesParams{
				GitServer: "my-server",
				Project:   "my-project",
				PerPage:   pointer.To(200),
			},
			wantPage:    1,
			wantPerPage: 100,
		},
		{
			name: "zero page is clamped to 1",
			params: models.ListPipelinesParams{
				GitServer: "my-server",
				Project:   "my-project",
				Page:      pointer.To(0),
			},
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name: "negative page is clamped to 1",
			params: models.ListPipelinesParams{
				GitServer: "my-server",
				Project:   "my-project",
				Page:      pointer.To(-5),
			},
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name: "negative perPage is clamped to 20",
			params: models.ListPipelinesParams{
				GitServer: "my-server",
				Project:   "my-project",
				PerPage:   pointer.To(-10),
			},
			wantPage:    1,
			wantPerPage: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubPipelineService{
				listResp: &models.PipelinesResponse{},
			}
			handler := NewPipelineHandler(stub)

			resp, err := handler.ListPipelines(context.Background(), ListPipelinesRequestObject{
				Params: tt.params,
			})

			require.NoError(t, err)
			assert.IsType(t, ListPipelines200JSONResponse{}, resp)
			assert.Equal(t, tt.params.GitServer, stub.gotListGitServer)
			assert.Equal(t, tt.params.Project, stub.gotListProject)
			assert.Equal(t, tt.wantPage, stub.gotListOpts.Page)
			assert.Equal(t, tt.wantPerPage, stub.gotListOpts.PerPage)
		})
	}
}

func TestPipelineHandlerListPipelinesWithFilters(t *testing.T) {
	stub := &stubPipelineService{
		listResp: &models.PipelinesResponse{},
	}
	handler := NewPipelineHandler(stub)

	ref := "main"
	status := models.ListPipelinesParamsStatus("success")

	resp, err := handler.ListPipelines(context.Background(), ListPipelinesRequestObject{
		Params: models.ListPipelinesParams{
			GitServer: "my-server",
			Project:   "my-project",
			Ref:       &ref,
			Status:    &status,
		},
	})

	require.NoError(t, err)
	assert.IsType(t, ListPipelines200JSONResponse{}, resp)
	require.NotNil(t, stub.gotListOpts.Ref)
	assert.Equal(t, "main", *stub.gotListOpts.Ref)
	require.NotNil(t, stub.gotListOpts.Status)
	assert.Equal(t, "success", *stub.gotListOpts.Status)
}

// --- Constructor and signature tests ---

func TestNewPipelineHandler(t *testing.T) {
	handler := NewPipelineHandler(nil)

	assert.NotNil(t, handler)
	assert.Nil(t, handler.pipelinesService)
}

func TestPipelineHandlerImplementsExpectedSignature(t *testing.T) {
	handler := &PipelineHandler{}

	triggerFn := handler.TriggerPipeline
	assert.NotNil(t, triggerFn)

	listFn := handler.ListPipelines
	assert.NotNil(t, listFn)
}

// --- ListPipelineJobs tests ---

func TestPipelineHandlerListPipelineJobsInvalidPipelineID(t *testing.T) {
	stub := &stubPipelineService{}
	handler := NewPipelineHandler(stub)

	resp, err := handler.ListPipelineJobs(context.Background(), ListPipelineJobsRequestObject{
		Params: models.ListPipelineJobsParams{GitServer: "gl", Project: "p", PipelineId: "abc"},
	})

	assert.Nil(t, resp)
	requireInvalidParam(t, err, "pipelineId")
	assert.Empty(t, stub.gotJobsGitServer, "service must not be called")
}

func TestPipelineHandlerListPipelineJobsSuccess(t *testing.T) {
	stub := &stubPipelineService{
		jobsResp: []models.PipelineJob{{Id: "16", Name: "build", Stage: "build", Status: "success"}},
	}
	handler := NewPipelineHandler(stub)

	resp, err := handler.ListPipelineJobs(context.Background(), ListPipelineJobsRequestObject{
		Params: models.ListPipelineJobsParams{GitServer: "gl", Project: "krci/app", PipelineId: "5"},
	})

	require.NoError(t, err)

	jobsResp, ok := resp.(ListPipelineJobs200JSONResponse)
	require.True(t, ok, "expected ListPipelineJobs200JSONResponse")
	assert.Equal(t, "gl", stub.gotJobsGitServer)
	assert.Equal(t, "krci/app", stub.gotJobsProject)
	assert.Equal(t, 5, stub.gotJobsPipelineID)
	require.Len(t, jobsResp.Data, 1)
	assert.Equal(t, "build", jobsResp.Data[0].Name)
}

// --- GetPipelineJobTrace tests ---

func TestPipelineHandlerGetPipelineJobTraceInvalidJobID(t *testing.T) {
	stub := &stubPipelineService{}
	handler := NewPipelineHandler(stub)

	resp, err := handler.GetPipelineJobTrace(context.Background(), GetPipelineJobTraceRequestObject{
		Params: models.GetPipelineJobTraceParams{GitServer: "gl", Project: "p", JobId: "nope"},
	})

	assert.Nil(t, resp)
	requireInvalidParam(t, err, "jobId")
	assert.Empty(t, stub.gotTraceGitServer, "service must not be called")
}

func TestPipelineHandlerGetPipelineJobTraceSuccess(t *testing.T) {
	stub := &stubPipelineService{traceResp: "hello log"}
	handler := NewPipelineHandler(stub)

	resp, err := handler.GetPipelineJobTrace(context.Background(), GetPipelineJobTraceRequestObject{
		Params: models.GetPipelineJobTraceParams{GitServer: "gl", Project: "krci/app", JobId: "23"},
	})

	require.NoError(t, err)

	traceResp, ok := resp.(GetPipelineJobTrace200JSONResponse)
	require.True(t, ok, "expected GetPipelineJobTrace200JSONResponse")
	assert.Equal(t, 23, stub.gotTraceJobID)
	assert.Equal(t, "23", traceResp.JobId)
	assert.Equal(t, "hello log", traceResp.Content)
}
