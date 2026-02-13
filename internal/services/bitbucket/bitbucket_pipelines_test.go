package bitbucket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

func TestNormalizeBitbucketPipelineStatus(t *testing.T) {
	tests := []struct {
		name       string
		stateName  string
		resultName string
		want       models.PipelineStatus
	}{
		{name: "PENDING maps to pending", stateName: "PENDING", resultName: "", want: models.PipelineStatusPending},
		{name: "IN_PROGRESS maps to running", stateName: "IN_PROGRESS", resultName: "", want: models.PipelineStatusRunning},
		{
			name: "COMPLETED+SUCCESSFUL maps to success", stateName: "COMPLETED",
			resultName: "SUCCESSFUL", want: models.PipelineStatusSuccess,
		},
		{
			name: "COMPLETED+FAILED maps to failed", stateName: "COMPLETED",
			resultName: "FAILED", want: models.PipelineStatusFailed,
		},
		{
			name: "COMPLETED+ERROR maps to failed", stateName: "COMPLETED",
			resultName: "ERROR", want: models.PipelineStatusFailed,
		},
		{
			name: "COMPLETED+STOPPED maps to cancelled", stateName: "COMPLETED",
			resultName: "STOPPED", want: models.PipelineStatusCancelled,
		},
		{
			name: "COMPLETED+EXPIRED maps to cancelled", stateName: "COMPLETED",
			resultName: "EXPIRED", want: models.PipelineStatusCancelled,
		},
		{
			name: "COMPLETED+unknown defaults to failed", stateName: "COMPLETED",
			resultName: "SOMETHING", want: models.PipelineStatusFailed,
		},
		{name: "HALTED maps to manual", stateName: "HALTED", resultName: "", want: models.PipelineStatusManual},
		{name: "PAUSED maps to manual", stateName: "PAUSED", resultName: "", want: models.PipelineStatusManual},
		{name: "unknown defaults to pending", stateName: "UNKNOWN", resultName: "", want: models.PipelineStatusPending},
		{name: "empty defaults to pending", stateName: "", resultName: "", want: models.PipelineStatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeBitbucketPipelineStatus(tt.stateName, tt.resultName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeBitbucketPipelineTrigger(t *testing.T) {
	tests := []struct {
		name        string
		triggerName string
		want        models.PipelineSource
	}{
		{name: "PUSH", triggerName: "PUSH", want: models.PipelineSourcePush},
		{name: "PULL_REQUEST maps to merge_request", triggerName: "PULL_REQUEST", want: models.PipelineSourceMergeRequest},
		{name: "SCHEDULE", triggerName: "SCHEDULE", want: models.PipelineSourceSchedule},
		{name: "MANUAL", triggerName: "MANUAL", want: models.PipelineSourceManual},
		{name: "TRIGGER maps to trigger", triggerName: "TRIGGER", want: models.PipelineSourceTrigger},
		{name: "API maps to trigger", triggerName: "API", want: models.PipelineSourceTrigger},
		{name: "unknown maps to other", triggerName: "UNKNOWN", want: models.PipelineSourceOther},
		{name: "empty maps to other", triggerName: "", want: models.PipelineSourceOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeBitbucketPipelineTrigger(tt.triggerName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapPipelineStatusToBitbucketQuery(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{name: "pending", status: "pending", want: `state.name="PENDING"`},
		{name: "running", status: "running", want: `state.name="IN_PROGRESS"`},
		{name: "success", status: "success", want: `state.result.name="SUCCESSFUL"`},
		{name: "failed", status: "failed", want: `state.result.name="FAILED"`},
		{name: "cancelled", status: "cancelled", want: `state.result.name="STOPPED"`},
		{name: "manual", status: "manual", want: `state.name="HALTED"`},
		{name: "unknown returns empty", status: "unknown", want: ""},
		{name: "empty returns empty", status: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPipelineStatusToBitbucketQuery(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBitbucketServiceListPipelinesFieldMapping(t *testing.T) {
	pipelinesJSON := `{
		"size": 1,
		"page": 1,
		"pagelen": 20,
		"values": [{
			"uuid": "{pipeline-uuid-123}",
			"build_number": 42,
			"state": {
				"name": "COMPLETED",
				"result": {"name": "SUCCESSFUL"}
			},
			"target": {
				"ref_type": "branch",
				"ref_name": "main",
				"commit": {"hash": "abc123def456"}
			},
			"trigger": {"name": "PUSH"},
			"created_on": "2026-01-15T10:30:00.123456+00:00",
			"completed_on": "2026-01-15T10:35:00.654321+00:00",
			"links": {
				"html": {"href": "https://bitbucket.org/owner/repo/pipelines/results/42"}
			}
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(pipelinesJSON))
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}
	token := testBitbucketToken()

	result, err := svc.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: token},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)

	p := result.Data[0]
	assert.Equal(t, "pipeline-uuid-123", p.Id)
	assert.Equal(t, models.PipelineStatusSuccess, p.Status)
	assert.Equal(t, "main", p.Ref)
	assert.Equal(t, "abc123def456", p.Sha)
	assert.Equal(t, "https://bitbucket.org/owner/repo/pipelines/results/42", p.WebUrl)

	// Source mapping
	require.NotNil(t, p.Source)
	assert.Equal(t, models.PipelineSourcePush, *p.Source)

	// ProjectId should be nil for Bitbucket
	assert.Nil(t, p.ProjectId)

	// Timestamps
	expectedCreatedAt, _ := time.Parse(time.RFC3339Nano, "2026-01-15T10:30:00.123456+00:00")
	assert.True(t, expectedCreatedAt.Equal(p.CreatedAt), "CreatedAt should match")
	require.NotNil(t, p.UpdatedAt)

	expectedUpdatedAt, _ := time.Parse(time.RFC3339Nano, "2026-01-15T10:35:00.654321+00:00")
	assert.True(t, expectedUpdatedAt.Equal(*p.UpdatedAt), "UpdatedAt should match")

	// Pagination
	assert.Equal(t, 1, result.Pagination.Total)
	require.NotNil(t, result.Pagination.Page)
	assert.Equal(t, 1, *result.Pagination.Page)
	require.NotNil(t, result.Pagination.PerPage)
	assert.Equal(t, 20, *result.Pagination.PerPage)
}

func TestBitbucketServiceListPipelinesMultipleStatuses(t *testing.T) {
	pipelinesJSON := `{
		"size": 3,
		"page": 1,
		"pagelen": 20,
		"values": [
			{
				"uuid": "{p1}", "build_number": 1,
				"state": {"name": "IN_PROGRESS"},
				"target": {"ref_name": "main", "commit": {"hash": "aaa"}},
				"trigger": {"name": "PUSH"},
				"created_on": "2026-01-15T10:00:00.000000+00:00",
				"links": {"html": {"href": "https://bb.org/p/1"}}
			},
			{
				"uuid": "{p2}", "build_number": 2,
				"state": {"name": "COMPLETED", "result": {"name": "FAILED"}},
				"target": {"ref_name": "feature", "commit": {"hash": "bbb"}},
				"trigger": {"name": "PULL_REQUEST"},
				"created_on": "2026-01-15T11:00:00.000000+00:00",
				"links": {"html": {"href": "https://bb.org/p/2"}}
			},
			{
				"uuid": "{p3}", "build_number": 3,
				"state": {"name": "COMPLETED", "result": {"name": "STOPPED"}},
				"target": {"ref_name": "main", "commit": {"hash": "ccc"}},
				"trigger": {"name": "MANUAL"},
				"created_on": "2026-01-15T12:00:00.000000+00:00",
				"links": {"html": {"href": "https://bb.org/p/3"}}
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(pipelinesJSON))
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	result, err := svc.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: testBitbucketToken()},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.Len(t, result.Data, 3)

	assert.Equal(t, models.PipelineStatusRunning, result.Data[0].Status)
	assert.Equal(t, models.PipelineStatusFailed, result.Data[1].Status)
	assert.Equal(t, models.PipelineStatusCancelled, result.Data[2].Status)

	require.NotNil(t, result.Data[0].Source)
	assert.Equal(t, models.PipelineSourcePush, *result.Data[0].Source)
	require.NotNil(t, result.Data[1].Source)
	assert.Equal(t, models.PipelineSourceMergeRequest, *result.Data[1].Source)
	require.NotNil(t, result.Data[2].Source)
	assert.Equal(t, models.PipelineSourceManual, *result.Data[2].Source)
}

func TestBitbucketServiceListPipelinesFilterParams(t *testing.T) {
	var capturedReq *http.Request

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := bitbucketPipelinesResponse{Size: 0, Page: 1, Pagelen: 20, Values: []bitbucketPipeline{}}
		body, _ := json.Marshal(resp)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	ref := "main"
	status := "success"

	_, err := svc.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: testBitbucketToken()},
		models.PipelineListOptions{
			Ref:     &ref,
			Status:  &status,
			Page:    1,
			PerPage: 20,
		},
	)

	require.NoError(t, err)
	require.NotNil(t, capturedReq)

	q := capturedReq.URL.Query().Get("q")
	assert.Contains(t, q, `target.ref_name="main"`)
	assert.Contains(t, q, `state.result.name="SUCCESSFUL"`)
}

func TestBitbucketServiceListPipelinesRefSanitization(t *testing.T) {
	var capturedReq *http.Request

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := bitbucketPipelinesResponse{Size: 0, Page: 1, Pagelen: 20, Values: []bitbucketPipeline{}}
		body, _ := json.Marshal(resp)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	// Attempt query injection via double quotes in the ref value
	maliciousRef := `main" OR target.ref_name="develop`

	_, err := svc.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: testBitbucketToken()},
		models.PipelineListOptions{
			Ref:     &maliciousRef,
			Page:    1,
			PerPage: 20,
		},
	)

	require.NoError(t, err)
	require.NotNil(t, capturedReq)

	q := capturedReq.URL.Query().Get("q")
	// The double quotes must be escaped, preventing query injection
	assert.Contains(t, q, `target.ref_name="main\" OR target.ref_name=\"develop"`)
	assert.NotContains(t, q, `target.ref_name="main" OR`)
}

func TestBitbucketServiceListPipelinesRefBackslashSanitization(t *testing.T) {
	var capturedReq *http.Request

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := bitbucketPipelinesResponse{Size: 0, Page: 1, Pagelen: 20, Values: []bitbucketPipeline{}}
		body, _ := json.Marshal(resp)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	// A trailing backslash would escape the closing quote without proper sanitization
	refWithBackslash := `main\`

	_, err := svc.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: testBitbucketToken()},
		models.PipelineListOptions{
			Ref:     &refWithBackslash,
			Page:    1,
			PerPage: 20,
		},
	)

	require.NoError(t, err)
	require.NotNil(t, capturedReq)

	q := capturedReq.URL.Query().Get("q")
	// The backslash must be escaped to \\, preventing it from escaping the closing quote
	assert.Contains(t, q, `target.ref_name="main\\"`)
}

func TestBitbucketServiceListPipelinesNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"Repository not found"}}`))
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	result, err := svc.ListPipelines(
		context.Background(),
		"nonexistent/project",
		krci.GitServerSettings{Token: testBitbucketToken()},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestBitbucketServiceListPipelinesUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Unauthorized"}}`))
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	result, err := svc.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: testBitbucketToken()},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestBitbucketServiceListPipelinesPagination(t *testing.T) {
	var capturedReq *http.Request

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := bitbucketPipelinesResponse{Size: 50, Page: 3, Pagelen: 10, Values: []bitbucketPipeline{}}
		body, _ := json.Marshal(resp)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	result, err := svc.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: testBitbucketToken()},
		models.PipelineListOptions{Page: 3, PerPage: 10},
	)

	require.NoError(t, err)
	require.NotNil(t, capturedReq)

	assert.Equal(t, "3", capturedReq.URL.Query().Get("page"))
	assert.Equal(t, "10", capturedReq.URL.Query().Get("pagelen"))
	assert.Equal(t, "-created_on", capturedReq.URL.Query().Get("sort"))

	assert.Equal(t, 50, result.Pagination.Total)
	require.NotNil(t, result.Pagination.Page)
	assert.Equal(t, 3, *result.Pagination.Page)
	require.NotNil(t, result.Pagination.PerPage)
	assert.Equal(t, 10, *result.Pagination.PerPage)
}

func TestBitbucketServiceListPipelinesInvalidProject(t *testing.T) {
	svc := &BitbucketService{httpClient: resty.New()}

	result, err := svc.ListPipelines(
		context.Background(),
		"noslash",
		krci.GitServerSettings{Token: testBitbucketToken()},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid project format")
}

func TestBitbucketServiceListPipelinesSkippedStatus(t *testing.T) {
	svc := &BitbucketService{httpClient: resty.New()}

	status := "skipped"

	result, err := svc.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: testBitbucketToken()},
		models.PipelineListOptions{
			Status:  &status,
			Page:    1,
			PerPage: 20,
		},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Data)
	assert.Equal(t, 0, result.Pagination.Total)
}

func TestBitbucketServiceListPipelinesInvalidToken(t *testing.T) {
	svc := NewBitbucketProvider()

	_, err := svc.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: "not-valid-base64!!!"},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode bitbucket token")
}

func TestBitbucketServiceListPipelinesInvalidTimestamp(t *testing.T) {
	pipelinesJSON := `{
		"size": 1,
		"page": 1,
		"pagelen": 20,
		"values": [{
			"uuid": "{p1}",
			"build_number": 1,
			"state": {"name": "COMPLETED", "result": {"name": "SUCCESSFUL"}},
			"target": {"ref_name": "main", "commit": {"hash": "abc"}},
			"trigger": {"name": "PUSH"},
			"created_on": "not-a-timestamp",
			"links": {"html": {"href": "https://bb.org/p/1"}}
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(pipelinesJSON))
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	_, err := svc.ListPipelines(
		context.Background(),
		"owner/repo",
		krci.GitServerSettings{Token: testBitbucketToken()},
		models.PipelineListOptions{Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse created_on time")
}

func TestBitbucketServiceTriggerPipelineUnsupported(t *testing.T) {
	svc := NewBitbucketProvider()

	result, err := svc.TriggerPipeline(
		context.Background(),
		"owner/repo",
		"main",
		nil,
		krci.GitServerSettings{Token: testBitbucketToken()},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not supported for Bitbucket")
}
