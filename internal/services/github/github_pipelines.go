package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/go-github/v72/github"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/common"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

const (
	ghConclusionSuccess   = "success"
	ghConclusionCancelled = "cancelled"
	ghConclusionSkipped   = "skipped"
)

// ListPipelines returns workflow runs for a GitHub repository.
func (g *GitHubProvider) ListPipelines(
	ctx context.Context,
	project string,
	settings krci.GitServerSettings,
	opts models.PipelineListOptions,
) (*models.PipelinesResponse, error) {
	owner, repo, err := common.SplitProject(project)
	if err != nil {
		return nil, err
	}

	client := github.NewClient(g.httpClient).WithAuthToken(settings.Token)

	ghOpts := &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{
			Page:    opts.Page,
			PerPage: opts.PerPage,
		},
	}

	if opts.Ref != nil {
		ghOpts.Branch = *opts.Ref
	}

	if opts.Status != nil {
		ghStatus := mapPipelineStatusToGitHub(*opts.Status)
		if ghStatus != nil {
			ghOpts.Status = *ghStatus
		}
	}

	workflowRuns, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, ghOpts)
	if err != nil {
		ghErr := &github.ErrorResponse{}
		if errors.As(err, &ghErr) {
			if ghErr.Response.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("project %s: %w", project, gferrors.ErrNotFound)
			}

			if ghErr.Response.StatusCode == http.StatusUnauthorized {
				return nil, fmt.Errorf("invalid credentials: %w", gferrors.ErrUnauthorized)
			}
		}

		return nil, fmt.Errorf("failed to list pipelines for %s: %w", project, err)
	}

	result := make([]models.Pipeline, 0, len(workflowRuns.WorkflowRuns))

	for _, run := range workflowRuns.WorkflowRuns {
		var createdAt time.Time
		if run.CreatedAt != nil {
			createdAt = run.CreatedAt.Time
		}

		pipeline := models.Pipeline{
			Id:        strconv.FormatInt(run.GetID(), 10),
			Status:    normalizeGitHubWorkflowRunStatus(run.GetStatus(), run.GetConclusion()),
			Ref:       run.GetHeadBranch(),
			Sha:       run.GetHeadSHA(),
			WebUrl:    run.GetHTMLURL(),
			CreatedAt: createdAt,
		}

		if run.UpdatedAt != nil {
			updatedAt := run.UpdatedAt.Time
			pipeline.UpdatedAt = &updatedAt
		}

		if run.Repository != nil && run.Repository.GetID() != 0 {
			projectID := strconv.FormatInt(run.Repository.GetID(), 10)
			pipeline.ProjectId = &projectID
		}

		if run.GetEvent() != "" {
			source := normalizeGitHubWorkflowRunEvent(run.GetEvent())
			pipeline.Source = &source
		}

		result = append(result, pipeline)
	}

	total := workflowRuns.GetTotalCount()

	return &models.PipelinesResponse{
		Data: result,
		Pagination: models.Pagination{
			Total:   total,
			Page:    &opts.Page,
			PerPage: &opts.PerPage,
		},
	}, nil
}

// normalizeGitHubWorkflowRunStatus maps GitHub workflow run status and conclusion
// to the unified pipeline status enum.
func normalizeGitHubWorkflowRunStatus(status, conclusion string) models.PipelineStatus {
	switch status {
	case "queued", "pending", "waiting", "requested":
		return models.PipelineStatusPending
	case "in_progress":
		return models.PipelineStatusRunning
	case "completed":
		switch conclusion {
		case ghConclusionSuccess, "neutral":
			return models.PipelineStatusSuccess
		case "failure", "timed_out", "startup_failure":
			return models.PipelineStatusFailed
		case ghConclusionCancelled:
			return models.PipelineStatusCancelled
		case ghConclusionSkipped:
			return models.PipelineStatusSkipped
		case "action_required":
			return models.PipelineStatusManual
		case "stale":
			return models.PipelineStatusCancelled
		default:
			return models.PipelineStatusFailed
		}
	default:
		return models.PipelineStatusPending
	}
}

// normalizeGitHubWorkflowRunEvent maps GitHub workflow run event types
// to the unified pipeline source enum.
func normalizeGitHubWorkflowRunEvent(event string) models.PipelineSource {
	switch event {
	case "push":
		return models.PipelineSourcePush
	case "pull_request", "pull_request_target":
		return models.PipelineSourceMergeRequest
	case "schedule":
		return models.PipelineSourceSchedule
	case "workflow_dispatch":
		return models.PipelineSourceManual
	case "repository_dispatch", "workflow_call":
		return models.PipelineSourceTrigger
	default:
		return models.PipelineSourceOther
	}
}

// mapPipelineStatusToGitHub maps a unified status filter string to the GitHub
// workflow run status/conclusion value used for API filtering.
func mapPipelineStatusToGitHub(status string) *string {
	var v string

	switch status {
	// NOTE: GitHub has multiple pending-equivalent statuses (queued, pending, waiting, requested)
	// but the API only accepts a single status filter value. "queued" is used as the best approximation.
	case "pending":
		v = "queued"
	case "running":
		v = "in_progress"
	case "success":
		v = ghConclusionSuccess
	case "failed":
		v = "failure"
	case "cancelled":
		v = ghConclusionCancelled
	case "skipped":
		v = ghConclusionSkipped
	case "manual":
		v = "action_required"
	default:
		return nil
	}

	return &v
}

// TriggerPipeline is not supported for GitHub.
func (g *GitHubProvider) TriggerPipeline(
	_ context.Context,
	_ string,
	_ string,
	_ []models.PipelineVariable,
	_ krci.GitServerSettings,
) (*models.PipelineResponse, error) {
	return nil, fmt.Errorf("trigger pipeline is not supported for GitHub")
}
