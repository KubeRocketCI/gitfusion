package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// --- TriggerPipeline tests ---

func TestPipelineHandlerTriggerPipelineValidation(t *testing.T) {
	handler := NewPipelineHandler(&stubPipelineService{})

	t.Run("missing gitServer returns 400", func(t *testing.T) {
		resp, err := handler.TriggerPipeline(context.Background(), TriggerPipelineRequestObject{
			Params: models.TriggerPipelineParams{
				GitServer: "",
				Project:   "my-project",
				Ref:       "main",
			},
		})

		require.NoError(t, err)
		assert.IsType(t, TriggerPipeline400JSONResponse{}, resp)
	})

	t.Run("missing project returns 400", func(t *testing.T) {
		resp, err := handler.TriggerPipeline(context.Background(), TriggerPipelineRequestObject{
			Params: models.TriggerPipelineParams{
				GitServer: "my-server",
				Project:   "",
				Ref:       "main",
			},
		})

		require.NoError(t, err)
		assert.IsType(t, TriggerPipeline400JSONResponse{}, resp)
	})

	t.Run("missing ref returns 400", func(t *testing.T) {
		resp, err := handler.TriggerPipeline(context.Background(), TriggerPipelineRequestObject{
			Params: models.TriggerPipelineParams{
				GitServer: "my-server",
				Project:   "my-project",
				Ref:       "",
			},
		})

		require.NoError(t, err)
		assert.IsType(t, TriggerPipeline400JSONResponse{}, resp)
	})
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

func TestPipelineHandlerTriggerErrResponse(t *testing.T) {
	handler := &PipelineHandler{}

	t.Run("bad request error returns 400", func(t *testing.T) {
		resp := handler.triggerErrResponse(fmt.Errorf("invalid project format: %w", gferrors.ErrBadRequest))

		errResp, ok := resp.(TriggerPipeline400JSONResponse)
		require.True(t, ok, "expected TriggerPipeline400JSONResponse")
		assert.Equal(t, fmt.Sprintf("%d", http.StatusBadRequest), errResp.Code)
		assert.Contains(t, errResp.Message, "bad_request")
	})

	t.Run("not found error returns 404", func(t *testing.T) {
		resp := handler.triggerErrResponse(fmt.Errorf("project not found: %w", gferrors.ErrNotFound))

		errResp, ok := resp.(TriggerPipeline404JSONResponse)
		require.True(t, ok, "expected TriggerPipeline404JSONResponse")
		assert.Equal(t, fmt.Sprintf("%d", http.StatusNotFound), errResp.Code)
		assert.Contains(t, errResp.Message, "not found")
	})

	t.Run("unauthorized error returns 401", func(t *testing.T) {
		resp := handler.triggerErrResponse(fmt.Errorf("token expired: %w", gferrors.ErrUnauthorized))

		errResp, ok := resp.(TriggerPipeline401JSONResponse)
		require.True(t, ok, "expected TriggerPipeline401JSONResponse")
		assert.Equal(t, fmt.Sprintf("%d", http.StatusUnauthorized), errResp.Code)
		assert.Contains(t, errResp.Message, "unauthorized")
	})

	t.Run("generic error returns 500 with generic message", func(t *testing.T) {
		resp := handler.triggerErrResponse(errors.New("something went wrong"))

		errResp, ok := resp.(TriggerPipeline500JSONResponse)
		require.True(t, ok, "expected TriggerPipeline500JSONResponse")
		assert.Equal(t, fmt.Sprintf("%d", http.StatusInternalServerError), errResp.Code)
		assert.Equal(t, "something went wrong", errResp.Message)
	})
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

func TestPipelineHandlerListErrResponse(t *testing.T) {
	handler := &PipelineHandler{}

	t.Run("unauthorized error returns 401", func(t *testing.T) {
		err := fmt.Errorf("token expired: %w", gferrors.ErrUnauthorized)

		resp := handler.listErrResponse(err)

		errResp, ok := resp.(ListPipelines401JSONResponse)
		require.True(t, ok, "expected ListPipelines401JSONResponse")
		assert.Equal(t, fmt.Sprintf("%d", http.StatusUnauthorized), errResp.Code)
		assert.Contains(t, errResp.Message, "unauthorized")
	})

	t.Run("bad request error returns 400", func(t *testing.T) {
		err := fmt.Errorf("invalid project format: %w", gferrors.ErrBadRequest)

		resp := handler.listErrResponse(err)

		errResp, ok := resp.(ListPipelines400JSONResponse)
		require.True(t, ok, "expected ListPipelines400JSONResponse")
		assert.Equal(t, fmt.Sprintf("%d", http.StatusBadRequest), errResp.Code)
		assert.Contains(t, errResp.Message, "bad_request")
	})

	t.Run("not found error returns 404", func(t *testing.T) {
		err := fmt.Errorf("project not found: %w", gferrors.ErrNotFound)

		resp := handler.listErrResponse(err)

		errResp, ok := resp.(ListPipelines404JSONResponse)
		require.True(t, ok, "expected ListPipelines404JSONResponse")
		assert.Equal(t, fmt.Sprintf("%d", http.StatusNotFound), errResp.Code)
		assert.Contains(t, errResp.Message, "not found")
	})

	t.Run("generic error returns 500 with generic message", func(t *testing.T) {
		resp := handler.listErrResponse(errors.New("something went wrong"))

		errResp, ok := resp.(ListPipelines500JSONResponse)
		require.True(t, ok, "expected ListPipelines500JSONResponse")
		assert.Equal(t, fmt.Sprintf("%d", http.StatusInternalServerError), errResp.Code)
		assert.Equal(t, "something went wrong", errResp.Message)
	})
}

// --- VisitResponse tests ---

func TestListPipelines200JSONResponseVisitResponse(t *testing.T) {
	page := 1
	perPage := 20

	resp := ListPipelines200JSONResponse{
		Data: []models.Pipeline{
			{
				Id:        "12345",
				Status:    models.PipelineStatusSuccess,
				Ref:       "main",
				Sha:       "abc123",
				WebUrl:    "https://gitlab.com/project/-/pipelines/12345",
				CreatedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		Pagination: models.Pagination{
			Total:   1,
			Page:    &page,
			PerPage: &perPage,
		},
	}

	w := httptest.NewRecorder()
	err := resp.VisitListPipelinesResponse(w)

	require.NoError(t, err)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), `"status":"success"`)
}

func TestListPipelines400JSONResponseVisitResponse(t *testing.T) {
	resp := ListPipelines400JSONResponse{
		Message: "bad request",
		Code:    "bad_request",
	}

	w := httptest.NewRecorder()
	err := resp.VisitListPipelinesResponse(w)

	require.NoError(t, err)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), `"message":"bad request"`)
}

func TestListPipelines401JSONResponseVisitResponse(t *testing.T) {
	resp := ListPipelines401JSONResponse{
		Message: "unauthorized",
		Code:    "401",
	}

	w := httptest.NewRecorder()
	err := resp.VisitListPipelinesResponse(w)

	require.NoError(t, err)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), `"message":"unauthorized"`)
}

func TestListPipelines500JSONResponseVisitResponse(t *testing.T) {
	resp := ListPipelines500JSONResponse{
		Message: "internal server error",
		Code:    "500",
	}

	w := httptest.NewRecorder()
	err := resp.VisitListPipelinesResponse(w)

	require.NoError(t, err)
	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), `"message":"internal server error"`)
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
