package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

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
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
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
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
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
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
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
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
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
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
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

		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
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
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
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

		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
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
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
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
