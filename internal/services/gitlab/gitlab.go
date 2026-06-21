package gitlab

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

// insecureSkipVerify reports whether TLS certificate verification is disabled for GitLab
// requests. It is controlled by GITFUSION_GITLAB_INSECURE_SKIP_VERIFY and defaults to false,
// keeping certificate verification on. Set it to "true" to connect to a GitLab instance that
// serves a self-signed certificate.
var insecureSkipVerify = os.Getenv("GITFUSION_GITLAB_INSECURE_SKIP_VERIFY") == "true"

var insecureWarnOnce sync.Once

// newGitlabClient builds a go-gitlab client for the given git server settings.
func newGitlabClient(settings krci.GitServerSettings) (*gitlab.Client, error) {
	opts := []gitlab.ClientOptionFunc{gitlab.WithBaseURL(settings.Url)}

	if insecureSkipVerify {
		opts = append(opts, gitlab.WithHTTPClient(newGitlabHTTPClient(0)))
	}

	return gitlab.NewClient(settings.Token, opts...)
}

// newGitlabHTTPClient builds an http.Client for GitLab requests. A timeout of 0 leaves the
// client without a deadline (used for the go-gitlab client, which makes long paginated
// calls); a non-zero timeout bounds one-shot calls such as the job-trace fetch.
func newGitlabHTTPClient(timeout time.Duration) *http.Client {
	c := &http.Client{Timeout: timeout}

	if insecureSkipVerify {
		insecureWarnOnce.Do(func() {
			slog.Warn("GITFUSION_GITLAB_INSECURE_SKIP_VERIFY is enabled — TLS certificate verification disabled")
		})

		// InsecureSkipVerify is enabled only on explicit opt-in via
		// GITFUSION_GITLAB_INSECURE_SKIP_VERIFY=true (self-signed GitLab certificates).
		c.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}

	return c
}

// maxTraceBytes caps the job trace read to prevent OOM on runaway logs (4 MiB).
const maxTraceBytes = 4 * 1024 * 1024

// traceRequestTimeout bounds a single job-trace fetch. The read itself is capped at
// maxTraceBytes, so this only guards against a hung or extremely slow connection.
const traceRequestTimeout = 30 * time.Second

// maxJobsTotal is the pagination cap for ListPipelineJobs (5 pages × 100).
const maxJobsTotal = 500

const (
	glStateOpened = "opened"
	glStateClosed = "closed"
	glStateMerged = "merged"

	glStatusPending  = "pending"
	glStatusRunning  = "running"
	glStatusSuccess  = "success"
	glStatusFailed   = "failed"
	glStatusCanceled = "canceled"
	glStatusSkipped  = "skipped"
	glStatusManual   = "manual"
)

type GitlabProvider struct{}

func NewGitlabProvider() *GitlabProvider {
	return &GitlabProvider{}
}

func (g *GitlabProvider) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
) (*models.Repository, error) {
	client, err := newGitlabClient(settings)
	if err != nil {
		return nil, err
	}

	repository, _, err := client.Projects.GetProject(
		fmt.Sprintf("%s/%s", owner, repo),
		nil,
		gitlab.WithContext(ctx),
	)
	if err != nil {
		if errors.Is(err, gitlab.ErrNotFound) {
			return nil, fmt.Errorf("repository %s/%s %w", owner, repo, gferrors.ErrNotFound)
		}

		return nil, err
	}

	return convertGitlabRepoToRepository(repository), nil
}

func (g *GitlabProvider) ListRepositories(
	ctx context.Context,
	owner string,
	settings krci.GitServerSettings,
	listOptions models.ListOptions,
) ([]models.Repository, error) {
	client, err := newGitlabClient(settings)
	if err != nil {
		return nil, err
	}

	it := gitlab.Scan2(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Project, *gitlab.Response, error) {
		return client.Groups.ListGroupProjects(
			owner,
			newGitlabRepositoryListByOrgOptions(listOptions),
			gitlab.WithContext(ctx),
		)
	})

	result := make([]models.Repository, 0)

	for repo, err := range it {
		if err != nil {
			if errors.Is(err, gitlab.ErrNotFound) {
				return nil, fmt.Errorf("owner %s %w", owner, gferrors.ErrNotFound)
			}

			return nil, err
		}

		result = append(result, *convertGitlabRepoToRepository(repo))
	}

	return result, nil
}

// ListUserOrganizations returns organizations for the authenticated user
func (g *GitlabProvider) ListUserOrganizations(
	ctx context.Context,
	settings krci.GitServerSettings,
) ([]models.Organization, error) {
	client, err := newGitlabClient(settings)
	if err != nil {
		return nil, err
	}

	it := gitlab.Scan2(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Group, *gitlab.Response, error) {
		return client.Groups.ListGroups(&gitlab.ListGroupsOptions{}, gitlab.WithContext(ctx), p)
	})

	result := make([]models.Organization, 0)

	for group, err := range it {
		if err != nil {
			return nil, fmt.Errorf("failed to list groups: %w", err)
		}

		org := models.Organization{
			Id:   strconv.Itoa(group.ID),
			Name: group.FullPath,
		}
		if group.AvatarURL != "" {
			org.AvatarUrl = &group.AvatarURL
		}

		result = append(result, org)
	}

	return result, nil
}

// ListBranches implements BranchesProvider for GitlabService.
func (g *GitlabProvider) ListBranches(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
	_ models.ListOptions,
) ([]models.Branch, error) {
	client, err := newGitlabClient(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	it := gitlab.Scan2(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Branch, *gitlab.Response, error) {
		return client.Branches.ListBranches(
			fmt.Sprintf("%s/%s", owner, repo),
			&gitlab.ListBranchesOptions{},
			gitlab.WithContext(ctx),
			p,
		)
	})

	result := make([]models.Branch, 0)

	for b, err := range it {
		if err != nil {
			if errors.Is(err, gitlab.ErrNotFound) {
				return nil, fmt.Errorf("project %s/%s: %w", owner, repo, gferrors.ErrNotFound)
			}

			return nil, fmt.Errorf("failed to list branches for %s/%s: %w", owner, repo, err)
		}

		result = append(result, models.Branch{
			Name: b.Name,
		})
	}

	return result, nil
}

// TriggerPipeline triggers a CI/CD pipeline in GitLab
func (g *GitlabProvider) TriggerPipeline(
	ctx context.Context,
	project string,
	ref string,
	variables []models.PipelineVariable,
	settings krci.GitServerSettings,
) (*models.PipelineResponse, error) {
	client, err := newGitlabClient(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	// Create pipeline
	opts := &gitlab.CreatePipelineOptions{
		Ref:       gitlab.Ptr(ref),
		Variables: convertToPipelineVariables(variables),
	}

	pipeline, resp, err := client.Pipelines.CreatePipeline(
		project,
		opts,
		gitlab.WithContext(ctx),
	)

	if err != nil {
		if errors.Is(err, gitlab.ErrNotFound) || (resp != nil && resp.StatusCode == http.StatusNotFound) {
			return nil, fmt.Errorf("project %s or ref %s: %w", project, ref, gferrors.ErrNotFound)
		}

		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return nil, fmt.Errorf("invalid credentials: %w", gferrors.ErrUnauthorized)
		}

		return nil, fmt.Errorf("create pipeline for %s ref %s: %w", project, ref, err)
	}

	result := &models.PipelineResponse{
		Id:     pipeline.ID,
		WebUrl: pipeline.WebURL,
		Status: pipeline.Status,
		Ref:    pipeline.Ref,
	}
	if pipeline.SHA != "" {
		result.Sha = &pipeline.SHA
	}

	return result, nil
}

// ListPipelines lists CI/CD pipelines for a GitLab project.
func (g *GitlabProvider) ListPipelines(
	ctx context.Context,
	project string,
	settings krci.GitServerSettings,
	opts models.PipelineListOptions,
) (*models.PipelinesResponse, error) {
	client, err := newGitlabClient(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	glOpts := &gitlab.ListProjectPipelinesOptions{
		ListOptions: gitlab.ListOptions{
			Page:    opts.Page,
			PerPage: opts.PerPage,
		},
	}

	if opts.Ref != nil {
		glOpts.Ref = opts.Ref
	}

	if opts.Status != nil {
		glStatus := mapPipelineStatusToGitLab(*opts.Status)
		if glStatus != nil {
			glOpts.Status = glStatus
		}
	}

	pipelines, resp, err := client.Pipelines.ListProjectPipelines(
		project,
		glOpts,
		gitlab.WithContext(ctx),
	)
	if err != nil {
		if errors.Is(err, gitlab.ErrNotFound) || (resp != nil && resp.StatusCode == http.StatusNotFound) {
			return nil, fmt.Errorf("project %s: %w", project, gferrors.ErrNotFound)
		}

		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return nil, fmt.Errorf("invalid credentials: %w", gferrors.ErrUnauthorized)
		}

		return nil, fmt.Errorf("failed to list pipelines for %s: %w", project, err)
	}

	total := resp.TotalItems

	result := make([]models.Pipeline, 0, len(pipelines))

	for _, p := range pipelines {
		var createdAt time.Time
		if p.CreatedAt != nil {
			createdAt = *p.CreatedAt
		}

		pipeline := models.Pipeline{
			Id:        strconv.Itoa(p.ID),
			Status:    normalizeGitLabPipelineStatus(p.Status),
			Ref:       p.Ref,
			Sha:       p.SHA,
			WebUrl:    p.WebURL,
			CreatedAt: createdAt,
		}

		if p.ProjectID != 0 {
			projectID := strconv.Itoa(p.ProjectID)
			pipeline.ProjectId = &projectID
		}

		if p.Source != "" {
			source := normalizeGitLabPipelineSource(p.Source)
			pipeline.Source = &source
		}

		if p.UpdatedAt != nil {
			pipeline.UpdatedAt = p.UpdatedAt
		}

		result = append(result, pipeline)
	}

	return &models.PipelinesResponse{
		Data: result,
		Pagination: models.Pagination{
			Total:   total,
			Page:    &opts.Page,
			PerPage: &opts.PerPage,
		},
	}, nil
}

// ListPipelineJobs lists the jobs of a GitLab CI pipeline, ordered by job ID ascending.
// It fetches up to maxJobsTotal jobs using autopagination; a warning is logged if the cap is hit.
func (g *GitlabProvider) ListPipelineJobs(
	ctx context.Context,
	project string,
	pipelineID int,
	settings krci.GitServerSettings,
) ([]models.PipelineJob, error) {
	client, err := newGitlabClient(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	it := gitlab.Scan2(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Job, *gitlab.Response, error) {
		return client.Jobs.ListPipelineJobs(
			project,
			pipelineID,
			&gitlab.ListJobsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}},
			gitlab.WithContext(ctx),
			p,
		)
	})

	rawJobs := make([]*gitlab.Job, 0, maxJobsTotal)

	for j, err := range it {
		if err != nil {
			return nil, mapGitLabJobsError(err, project, pipelineID)
		}

		rawJobs = append(rawJobs, j)

		if len(rawJobs) >= maxJobsTotal {
			slog.Warn("ListPipelineJobs reached pagination cap; some jobs may be omitted",
				"project", project,
				"pipelineID", pipelineID,
				"cap", maxJobsTotal,
			)

			break
		}
	}

	// GitLab returns newest-first; present oldest-first (by numeric job ID) so stages
	// read top-to-bottom. Sort the raw jobs on their integer ID to avoid re-parsing
	// the stringified ID on every comparison.
	sort.SliceStable(rawJobs, func(i, k int) bool {
		return rawJobs[i].ID < rawJobs[k].ID
	})

	result := make([]models.PipelineJob, 0, len(rawJobs))
	for _, j := range rawJobs {
		result = append(result, mapGitLabJob(j))
	}

	return result, nil
}

// GetJobTrace returns the raw trace (log) text of a GitLab CI job and whether it was
// truncated to maxTraceBytes.
//
// We deliberately bypass go-gitlab's Jobs.GetTraceFile: that helper buffers the entire
// response into memory (bytes.Buffer) before returning a reader, so an io.LimitReader
// over its result would cap only the copied string, not the allocation — a multi-hundred-MB
// log would still be fully resident. Instead we issue the documented
// GET /projects/:id/jobs/:job_id/trace request and stream resp.Body through an
// io.LimitReader, so at most maxTraceBytes+1 bytes are ever read off the socket. The extra
// byte lets us detect truncation authoritatively rather than inferring it from length.
func (g *GitlabProvider) GetJobTrace(
	ctx context.Context,
	project string,
	jobID int,
	settings krci.GitServerSettings,
) (string, bool, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%d/trace",
		strings.TrimRight(settings.Url, "/"),
		gitlab.PathEscape(project),
		jobID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("failed to build job trace request for %s job %d: %w", project, jobID, err)
	}

	req.Header.Set("PRIVATE-TOKEN", settings.Token)

	resp, err := newGitlabHTTPClient(traceRequestTimeout).Do(req)
	if err != nil {
		return "", false, fmt.Errorf("failed to fetch job trace for %s job %d: %w", project, jobID, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", false, mapGitLabTraceStatus(resp.StatusCode, project, jobID)
	}

	// resp.Body streams, so the LimitReader stops the socket read at the cap; a runaway
	// log never fully allocates. We intentionally do not drain the remainder on truncation
	// (closing the body aborts the connection instead of downloading the rest).
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTraceBytes+1))
	if err != nil {
		return "", false, fmt.Errorf("failed to read job trace for %s job %d: %w", project, jobID, err)
	}

	if len(data) > maxTraceBytes {
		return string(data[:maxTraceBytes]), true, nil
	}

	return string(data), false, nil
}

// mapGitLabTraceStatus maps a job-trace HTTP status to a GitFusion sentinel error.
func mapGitLabTraceStatus(statusCode int, project string, jobID int) error {
	switch statusCode {
	case http.StatusNotFound:
		return fmt.Errorf("project %s or job %d: %w", project, jobID, gferrors.ErrNotFound)
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("invalid credentials: %w", gferrors.ErrUnauthorized)
	default:
		return fmt.Errorf("gitlab trace request failed for %s job %d: status %d", project, jobID, statusCode)
	}
}

// mapGitLabJob converts a go-gitlab Job to the unified PipelineJob model.
func mapGitLabJob(j *gitlab.Job) models.PipelineJob {
	job := models.PipelineJob{
		Id:     strconv.Itoa(j.ID),
		Name:   j.Name,
		Stage:  j.Stage,
		Status: j.Status,
	}

	if j.Ref != "" {
		ref := j.Ref
		job.Ref = &ref
	}

	if j.WebURL != "" {
		webURL := j.WebURL
		job.WebUrl = &webURL
	}

	// allow_failure is always present in the response (defaults to false).
	allowFailure := j.AllowFailure
	job.AllowFailure = &allowFailure

	if j.Duration != 0 {
		duration := float32(j.Duration)
		job.Duration = &duration
	}

	job.CreatedAt = j.CreatedAt
	job.StartedAt = j.StartedAt
	job.FinishedAt = j.FinishedAt

	if j.FailureReason != "" {
		reason := j.FailureReason
		job.FailureReason = &reason
	}

	return job
}

// mapGitLabJobsError maps go-gitlab errors to GitFusion sentinel errors for jobs/trace.
func mapGitLabJobsError(err error, project string, id int) error {
	// gitlab.Scan2 does not surface the *gitlab.Response, so recover the HTTP status
	// from the error itself — otherwise 401/403 from a paginated call (ListPipelineJobs)
	// would fall through to a generic 500 instead of the correct credentials error.
	statusCode := 0

	var errResp *gitlab.ErrorResponse
	if errors.As(err, &errResp) && errResp.Response != nil {
		statusCode = errResp.Response.StatusCode
	}

	if errors.Is(err, gitlab.ErrNotFound) || statusCode == http.StatusNotFound {
		return fmt.Errorf("project %s or id %d: %w", project, id, gferrors.ErrNotFound)
	}

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return fmt.Errorf("invalid credentials: %w", gferrors.ErrUnauthorized)
	}

	return fmt.Errorf("gitlab jobs request failed for %s id %d: %w", project, id, err)
}

// normalizeGitLabPipelineStatus maps GitLab pipeline status strings to the unified status enum.
func normalizeGitLabPipelineStatus(status string) models.PipelineStatus {
	switch status {
	case glStatusPending, "created", "waiting_for_resource", "preparing":
		return models.PipelineStatusPending
	case glStatusRunning:
		return models.PipelineStatusRunning
	case glStatusSuccess:
		return models.PipelineStatusSuccess
	case glStatusFailed:
		return models.PipelineStatusFailed
	case glStatusCanceled:
		return models.PipelineStatusCancelled
	case glStatusSkipped:
		return models.PipelineStatusSkipped
	case glStatusManual, "scheduled":
		return models.PipelineStatusManual
	default:
		return models.PipelineStatusPending
	}
}

// normalizeGitLabPipelineSource maps GitLab pipeline source strings to the unified source enum.
func normalizeGitLabPipelineSource(source string) models.PipelineSource {
	switch source {
	case "push":
		return models.PipelineSourcePush
	case "merge_request_event":
		return models.PipelineSourceMergeRequest
	case "schedule":
		return models.PipelineSourceSchedule
	case "web", "chat":
		return models.PipelineSourceManual
	case "trigger", "pipeline", "api":
		return models.PipelineSourceTrigger
	default:
		return models.PipelineSourceOther
	}
}

// mapPipelineStatusToGitLab maps the unified status filter to a GitLab BuildStateValue.
func mapPipelineStatusToGitLab(status string) *gitlab.BuildStateValue {
	var v gitlab.BuildStateValue

	switch status {
	case glStatusPending:
		v = gitlab.Pending
	case glStatusRunning:
		v = gitlab.Running
	case glStatusSuccess:
		v = gitlab.Success
	case glStatusFailed:
		v = gitlab.Failed
	case "cancelled":
		v = gitlab.Canceled
	case glStatusSkipped:
		v = gitlab.Skipped
	case glStatusManual:
		v = gitlab.Manual
	default:
		return nil
	}

	return &v
}

func convertGitlabRepoToRepository(repo *gitlab.Project) *models.Repository {
	if repo == nil {
		return nil
	}

	return &models.Repository{
		DefaultBranch: &repo.DefaultBranch,
		Description:   &repo.Description,
		Id:            strconv.Itoa(repo.ID),
		Name:          repo.Path,
		Owner:         &repo.Namespace.FullPath,
		Url:           &repo.WebURL,
		Visibility:    convertVisibility(repo.Visibility == gitlab.PrivateVisibility),
	}
}

func newGitlabRepositoryListByOrgOptions(listOptions models.ListOptions) *gitlab.ListGroupProjectsOptions {
	return &gitlab.ListGroupProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
		Search: listOptions.Name,
	}
}

func convertVisibility(isPrivate bool) *models.RepositoryVisibility {
	if isPrivate {
		visibility := models.RepositoryVisibilityPrivate

		return &visibility
	}

	visibility := models.RepositoryVisibilityPublic

	return &visibility
}

// ListPullRequests returns merge requests for a GitLab project.
func (g *GitlabProvider) ListPullRequests(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
	opts models.PullRequestListOptions,
) (*models.PullRequestsResponse, error) {
	client, err := newGitlabClient(settings)
	if err != nil {
		return nil, err
	}

	glState := mapPullRequestStateToGitLab(opts.State)

	mrs, resp, err := client.MergeRequests.ListProjectMergeRequests(
		fmt.Sprintf("%s/%s", owner, repo),
		&gitlab.ListProjectMergeRequestsOptions{
			State: gitlab.Ptr(glState),
			ListOptions: gitlab.ListOptions{
				Page:    opts.Page,
				PerPage: opts.PerPage,
			},
		},
		gitlab.WithContext(ctx),
	)
	if err != nil {
		if errors.Is(err, gitlab.ErrNotFound) || (resp != nil && resp.StatusCode == http.StatusNotFound) {
			return nil, fmt.Errorf("project %s/%s: %w", owner, repo, gferrors.ErrNotFound)
		}

		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return nil, fmt.Errorf("invalid credentials: %w", gferrors.ErrUnauthorized)
		}

		return nil, fmt.Errorf("failed to list merge requests for %s/%s: %w", owner, repo, err)
	}

	total := resp.TotalItems

	result := make([]models.PullRequest, 0, len(mrs))

	for _, mr := range mrs {
		var createdAt, updatedAt time.Time
		if mr.CreatedAt != nil {
			createdAt = *mr.CreatedAt
		}

		if mr.UpdatedAt != nil {
			updatedAt = *mr.UpdatedAt
		}

		pr := models.PullRequest{
			Id:           strconv.Itoa(mr.ID),
			Number:       mr.IID,
			Title:        mr.Title,
			State:        normalizeGitLabMRState(mr.State),
			SourceBranch: mr.SourceBranch,
			TargetBranch: mr.TargetBranch,
			Url:          mr.WebURL,
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
			Draft:        &mr.Draft,
		}

		if mr.Description != "" {
			pr.Description = &mr.Description
		}

		if mr.SHA != "" {
			pr.CommitSha = &mr.SHA
		}

		if mr.Author != nil {
			pr.Author = &models.Owner{
				Id:   strconv.Itoa(mr.Author.ID),
				Name: mr.Author.Username,
			}

			if mr.Author.AvatarURL != "" {
				pr.Author.AvatarUrl = &mr.Author.AvatarURL
			}
		}

		result = append(result, pr)
	}

	return &models.PullRequestsResponse{
		Data: result,
		Pagination: models.Pagination{
			Total:   total,
			Page:    &opts.Page,
			PerPage: &opts.PerPage,
		},
	}, nil
}

func mapPullRequestStateToGitLab(state string) string {
	switch state {
	case "open":
		return glStateOpened
	case "closed":
		return glStateClosed
	case "merged":
		return glStateMerged
	case "all":
		return "all"
	default:
		return glStateOpened
	}
}

func normalizeGitLabMRState(state string) models.PullRequestState {
	switch state {
	case glStateOpened:
		return models.PullRequestStateOpen
	case glStateMerged:
		return models.PullRequestStateMerged
	case glStateClosed:
		return models.PullRequestStateClosed
	default:
		return models.PullRequestStateOpen
	}
}

func convertToPipelineVariables(variables []models.PipelineVariable) *[]*gitlab.PipelineVariableOptions {
	if len(variables) == 0 {
		return nil
	}

	vars := make([]*gitlab.PipelineVariableOptions, len(variables))
	for i, v := range variables {
		vars[i] = &gitlab.PipelineVariableOptions{
			Key:   gitlab.Ptr(v.Key),
			Value: gitlab.Ptr(v.Value),
		}

		if v.VariableType != nil {
			varType := gitlab.VariableTypeValue(*v.VariableType)
			vars[i].VariableType = &varType
		}
	}

	return &vars
}
