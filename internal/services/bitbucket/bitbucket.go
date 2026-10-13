package bitbucket

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/ktrysmt/go-bitbucket"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/common"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	bitbucketpkg "github.com/KubeRocketCI/gitfusion/pkg/bitbucket"
)

// defaultBitbucketAPIURL is the Bitbucket Cloud REST API base URL for requests sent without go-bitbucket.
// go-bitbucket clients from newClient default to the same URL; BITBUCKET_API_BASE_URL overrides it for them only.
// A configurable API URL (e.g. settings.Url for Bitbucket Data Center): change this constant and newClient
// (bitbucket.NewBasicAuthWithBaseUrlStr) together.
const defaultBitbucketAPIURL = "https://api.bitbucket.org/2.0"

// isBitbucketNotFound checks whether the error from the go-bitbucket library
// indicates a 404 response. The library returns *UnexpectedResponseStatusError
// with Status set to the HTTP status text (e.g. "404 Not Found").
func isBitbucketNotFound(err error) bool {
	var statusErr *bitbucket.UnexpectedResponseStatusError
	if errors.As(err, &statusErr) {
		return strings.HasPrefix(statusErr.Status, "404")
	}

	return false
}

type BitbucketService struct {
	httpClient *resty.Client
}

// NewBitbucketProvider returns a service shared by all git servers.
// Keep the HTTP client without a cookie jar: cookies must not cross credentials.
func NewBitbucketProvider() *BitbucketService {
	return &BitbucketService{
		httpClient: resty.New().SetCookieJar(nil),
	}
}

// newClient returns a go-bitbucket client for the credentials in settings.Token.
// Create every go-bitbucket client here: it sends requests through the service HTTP client.
func (b *BitbucketService) newClient(settings krci.GitServerSettings) (*bitbucket.Client, error) {
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bitbucket token: %w", err)
	}

	client, err := bitbucket.NewBasicAuth(username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to create bitbucket client: %w", err)
	}

	client.HttpClient = b.httpClient.GetClient()

	return client, nil
}

func (b *BitbucketService) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
) (*models.Repository, error) {
	client, err := b.newClient(settings)
	if err != nil {
		return nil, err
	}

	repoOptions := &bitbucket.RepositoryOptions{
		Owner:    owner,
		RepoSlug: repo,
	}

	repository, err := client.Repositories.Repository.Get(repoOptions)
	if err != nil {
		if isBitbucketNotFound(err) {
			return nil, fmt.Errorf("repository %s/%s %w", owner, repo, gferrors.ErrNotFound)
		}

		return nil, fmt.Errorf("failed to get repository %s/%s: %w", owner, repo, err)
	}

	return convertBitbucketRepoToRepository(repository), nil
}

func (b *BitbucketService) ListRepositories(
	ctx context.Context,
	account string,
	settings krci.GitServerSettings,
	listOptions models.ListOptions,
) ([]models.Repository, error) {
	client, err := b.newClient(settings)
	if err != nil {
		return nil, err
	}

	repoOptions := &bitbucket.RepositoriesOptions{
		Owner:   account,
		Keyword: listOptions.Name,
	}

	repositories, err := client.Repositories.ListForAccount(repoOptions)
	if err != nil {
		if isBitbucketNotFound(err) {
			return nil, fmt.Errorf("account %s: %w", account, gferrors.ErrNotFound)
		}

		return nil, fmt.Errorf("failed to list repositories for account %s: %w", account, err)
	}

	result := make([]models.Repository, 0, len(repositories.Items))
	for _, repo := range repositories.Items {
		result = append(result, *convertBitbucketRepoToRepository(&repo))
	}

	return result, nil
}

type bitbucketWorkspacesResponse struct {
	Size    int                        `json:"size"`
	Page    int                        `json:"page"`
	Pagelen int                        `json:"pagelen"`
	Values  []bitbucketWorkspaceAccess `json:"values"`
}

// bitbucketWorkspaceAccess represents an item in the /user/workspaces response.
// The API returns workspace_access objects where the workspace data is nested
// under the "workspace" key.
type bitbucketWorkspaceAccess struct {
	Workspace bitbucketWorkspace `json:"workspace"`
}

type bitbucketWorkspace struct {
	UUID string `json:"uuid"`
	Slug string `json:"slug"`
}

// ListUserOrganizations returns workspaces for the authenticated user using a direct HTTP call
// to the Bitbucket REST API.
//
// Why direct HTTP instead of go-bitbucket client.Workspaces.List():
//   - The go-bitbucket library (v0.9.95) calls the deprecated /2.0/workspaces endpoint, which
//     Bitbucket removed as announced in:
//     https://developer.atlassian.com/cloud/bitbucket/changelog/#CHANGE-2770
//     https://developer.atlassian.com/cloud/bitbucket/changelog/#CHANGE-3022
//   - The correct endpoint is now /2.0/user/workspaces (requires authentication):
//     https://developer.atlassian.com/cloud/bitbucket/rest/api-group-workspaces/#api-user-workspaces-get
//   - The library has not been updated to support the new endpoint, so we call it directly
//     using the same go-resty HTTP client used by ListPullRequests and ListPipelines.
func (b *BitbucketService) ListUserOrganizations(
	ctx context.Context,
	settings krci.GitServerSettings,
) ([]models.Organization, error) {
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, err
	}

	apiURL := defaultBitbucketAPIURL + "/user/workspaces"

	var bbResp bitbucketWorkspacesResponse

	resp, err := b.httpClient.R().
		SetContext(ctx).
		SetBasicAuth(username, password).
		SetResult(&bbResp).
		Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}

	if resp.StatusCode() == http.StatusUnauthorized || resp.StatusCode() == http.StatusForbidden {
		return nil, fmt.Errorf("invalid credentials: %w", gferrors.ErrUnauthorized)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("failed to list workspaces: status %d, body: %s", resp.StatusCode(), resp.String())
	}

	result := make([]models.Organization, 0, len(bbResp.Values))

	for _, wa := range bbResp.Values {
		ws := wa.Workspace

		result = append(result, models.Organization{
			Id:   ws.UUID,
			Name: ws.Slug,
		})
	}

	return result, nil
}

// ListBranches implements BranchesProvider for BitbucketService.
func (b *BitbucketService) ListBranches(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
	_ models.ListOptions,
) ([]models.Branch, error) {
	client, err := b.newClient(settings)
	if err != nil {
		return nil, err
	}

	branchOptions := &bitbucket.RepositoryBranchOptions{
		Owner:    owner,
		RepoSlug: repo,
		Pagelen:  100,
	}

	scanBranches := bitbucketpkg.ScanBitbucketBranches(
		func(rbo *bitbucket.RepositoryBranchOptions) (*bitbucket.RepositoryBranches, error) {
			return client.Repositories.Repository.ListBranches(rbo)
		},
		branchOptions,
	)

	result := make([]models.Branch, 0)

	for b, err := range scanBranches {
		if err != nil {
			if isBitbucketNotFound(err) {
				return nil, fmt.Errorf("repository %s/%s: %w", owner, repo, gferrors.ErrNotFound)
			}

			return nil, fmt.Errorf("failed to list branches: %w", err)
		}

		result = append(result, models.Branch{
			Name: b.Name,
		})
	}

	return result, nil
}

type bitbucketPRResponse struct {
	Size    int           `json:"size"`
	Page    int           `json:"page"`
	Pagelen int           `json:"pagelen"`
	Values  []bitbucketPR `json:"values"`
}

type bitbucketPR struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	State       string `json:"state"`
	Description string `json:"description"`
	Draft       bool   `json:"draft"`

	Author struct {
		DisplayName string `json:"display_name"`
		UUID        string `json:"uuid"`
		Links       struct {
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
		} `json:"links"`
	} `json:"author"`

	Source struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
	} `json:"source"`

	Destination struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
	} `json:"destination"`

	Links struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`

	CreatedOn string `json:"created_on"`
	UpdatedOn string `json:"updated_on"`
}

// ListPullRequests returns pull requests for the given repository using the Bitbucket REST API.
// Direct HTTP (via go-resty) is used instead of go-bitbucket's PullRequests.Gets() because
// the library auto-paginates all pages into memory and lacks page/perPage control.
func (b *BitbucketService) ListPullRequests(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
	opts models.PullRequestListOptions,
) (*models.PullRequestsResponse, error) {
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bitbucket token: %w", err)
	}

	apiURL := fmt.Sprintf("%s/repositories/%s/%s/pullrequests",
		defaultBitbucketAPIURL, url.PathEscape(owner), url.PathEscape(repo))

	queryParams := url.Values{}
	queryParams.Set("page", strconv.Itoa(opts.Page))
	queryParams.Set("pagelen", strconv.Itoa(opts.PerPage))

	switch opts.State {
	case "open":
		queryParams.Set("state", "OPEN")
	case "closed":
		queryParams.Set("state", "DECLINED")
	case "merged":
		queryParams.Set("state", "MERGED")
	case "all":
		queryParams.Add("state", "OPEN")
		queryParams.Add("state", "MERGED")
		queryParams.Add("state", "DECLINED")
		queryParams.Add("state", "SUPERSEDED")
	default:
		queryParams.Set("state", "OPEN")
	}

	var bbResp bitbucketPRResponse

	resp, err := b.httpClient.R().
		SetContext(ctx).
		SetBasicAuth(username, password).
		SetQueryParamsFromValues(queryParams).
		SetResult(&bbResp).
		Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests for %s/%s: %w", owner, repo, err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("repository %s/%s: %w", owner, repo, gferrors.ErrNotFound)
	}

	if resp.StatusCode() == http.StatusUnauthorized || resp.StatusCode() == http.StatusForbidden {
		return nil, fmt.Errorf("invalid credentials: %w", gferrors.ErrUnauthorized)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("failed to list pull requests for %s/%s: status %d, body: %s",
			owner, repo, resp.StatusCode(), resp.String())
	}

	result := make([]models.PullRequest, 0, len(bbResp.Values))

	for _, pr := range bbResp.Values {
		state := convertBitbucketPRState(pr.State)

		createdAt, err := time.Parse(time.RFC3339Nano, pr.CreatedOn)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_on time %q: %w", pr.CreatedOn, err)
		}

		updatedAt, err := time.Parse(time.RFC3339Nano, pr.UpdatedOn)
		if err != nil {
			return nil, fmt.Errorf("failed to parse updated_on time %q: %w", pr.UpdatedOn, err)
		}

		author := &models.Owner{
			Id:   pr.Author.UUID,
			Name: pr.Author.DisplayName,
		}

		if pr.Author.Links.Avatar.Href != "" {
			avatarURL := pr.Author.Links.Avatar.Href
			author.AvatarUrl = &avatarURL
		}

		prModel := models.PullRequest{
			Id:           strconv.Itoa(pr.ID),
			Number:       pr.ID,
			Title:        pr.Title,
			State:        state,
			SourceBranch: pr.Source.Branch.Name,
			TargetBranch: pr.Destination.Branch.Name,
			Url:          pr.Links.HTML.Href,
			Author:       author,
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
			Draft:        &pr.Draft,
		}

		if pr.Description != "" {
			prModel.Description = &pr.Description
		}

		if pr.Source.Commit.Hash != "" {
			prModel.CommitSha = &pr.Source.Commit.Hash
		}

		result = append(result, prModel)
	}

	total := bbResp.Size

	return &models.PullRequestsResponse{
		Data: result,
		Pagination: models.Pagination{
			Total:   total,
			Page:    &opts.Page,
			PerPage: &opts.PerPage,
		},
	}, nil
}

func convertBitbucketPRState(state string) models.PullRequestState {
	switch state {
	case "OPEN":
		return models.PullRequestStateOpen
	case "MERGED":
		return models.PullRequestStateMerged
	case "DECLINED", "SUPERSEDED":
		return models.PullRequestStateClosed
	default:
		return models.PullRequestStateOpen
	}
}

func convertBitbucketRepoToRepository(repo *bitbucket.Repository) *models.Repository {
	// Extract owner username and URL from the generic map fields
	ownerUsername := ""
	if owner, ok := repo.Owner["username"].(string); ok {
		ownerUsername = owner
	}

	repoUrl := ""

	if html, ok := repo.Links["html"].(map[string]any); ok {
		if href, ok := html["href"].(string); ok {
			repoUrl = href
		}
	}

	return &models.Repository{
		DefaultBranch: &repo.Mainbranch.Name,
		Description:   &repo.Description,
		Id:            repo.Uuid,
		Name:          repo.Name,
		Owner:         &ownerUsername,
		Url:           &repoUrl,
	}
}

func decodeBitbucketToken(token string) (username, password string, err error) {
	decodedToken, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode token: %w", err)
	}

	basicAuth := strings.Split(string(decodedToken), ":")

	if len(basicAuth) != 2 {
		return "", "", fmt.Errorf("invalid token format")
	}

	return basicAuth[0], basicAuth[1], nil
}

type bitbucketPipelinesResponse struct {
	Size    int                 `json:"size"`
	Page    int                 `json:"page"`
	Pagelen int                 `json:"pagelen"`
	Values  []bitbucketPipeline `json:"values"`
}

type bitbucketPipeline struct {
	UUID        string `json:"uuid"`
	BuildNumber int    `json:"build_number"`

	State struct {
		Name   string `json:"name"`
		Result *struct {
			Name string `json:"name"`
		} `json:"result,omitempty"`
	} `json:"state"`

	Target struct {
		RefType string `json:"ref_type"`
		RefName string `json:"ref_name"`
		Commit  struct {
			Hash string `json:"hash"`
		} `json:"commit"`
	} `json:"target"`

	Trigger struct {
		Name string `json:"name"`
	} `json:"trigger"`

	CreatedOn   string `json:"created_on"`
	CompletedOn string `json:"completed_on"`

	Links struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
}

// ListPipelines returns pipelines for a Bitbucket repository using the REST API.
func (b *BitbucketService) ListPipelines(
	ctx context.Context,
	project string,
	settings krci.GitServerSettings,
	opts models.PipelineListOptions,
) (*models.PipelinesResponse, error) {
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bitbucket token: %w", err)
	}

	workspace, repoSlug, err := common.SplitProject(project)
	if err != nil {
		return nil, err
	}

	// Bitbucket has no "skipped" state.
	if opts.Status != nil && *opts.Status == "skipped" {
		return &models.PipelinesResponse{
			Data: []models.Pipeline{},
			Pagination: models.Pagination{
				Total:   0,
				Page:    &opts.Page,
				PerPage: &opts.PerPage,
			},
		}, nil
	}

	apiURL := fmt.Sprintf("%s/repositories/%s/%s/pipelines/",
		defaultBitbucketAPIURL, url.PathEscape(workspace), url.PathEscape(repoSlug))

	var bbResp bitbucketPipelinesResponse

	resp, err := b.httpClient.R().
		SetContext(ctx).
		SetBasicAuth(username, password).
		SetQueryParamsFromValues(bitbucketPipelinesQuery(opts)).
		SetResult(&bbResp).
		Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list pipelines for %s: %w", project, err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("project %s: %w", project, gferrors.ErrNotFound)
	}

	if resp.StatusCode() == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid credentials: %w", gferrors.ErrUnauthorized)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("failed to list pipelines for %s: status %d, body: %s",
			project, resp.StatusCode(), resp.String())
	}

	result := make([]models.Pipeline, 0, len(bbResp.Values))

	for i := range bbResp.Values {
		pipeline, err := convertBitbucketPipeline(&bbResp.Values[i])
		if err != nil {
			return nil, err
		}

		result = append(result, pipeline)
	}

	return &models.PipelinesResponse{
		Data: result,
		Pagination: models.Pagination{
			Total:   bbResp.Size,
			Page:    &opts.Page,
			PerPage: &opts.PerPage,
		},
	}, nil
}

// bitbucketPipelinesQuery builds the pipelines query: newest first, filtered by ref and status.
// Status "skipped" has no Bitbucket filter; ListPipelines returns before building the query.
func bitbucketPipelinesQuery(opts models.PipelineListOptions) url.Values {
	query := url.Values{}
	query.Set("page", strconv.Itoa(opts.Page))
	query.Set("pagelen", strconv.Itoa(opts.PerPage))
	query.Set("sort", "-created_on")

	var filters []string

	if opts.Ref != nil && *opts.Ref != "" {
		sanitizedRef := strings.ReplaceAll(*opts.Ref, `\`, `\\`)
		sanitizedRef = strings.ReplaceAll(sanitizedRef, `"`, `\"`)
		filters = append(filters, fmt.Sprintf(`target.ref_name="%s"`, sanitizedRef))
	}

	if opts.Status != nil {
		if statusFilter := mapPipelineStatusToBitbucketQuery(*opts.Status); statusFilter != "" {
			filters = append(filters, statusFilter)
		}
	}

	if len(filters) > 0 {
		query.Set("q", strings.Join(filters, " AND "))
	}

	return query
}

func convertBitbucketPipeline(p *bitbucketPipeline) (models.Pipeline, error) {
	var resultName string
	if p.State.Result != nil {
		resultName = p.State.Result.Name
	}

	createdAt, err := time.Parse(time.RFC3339Nano, p.CreatedOn)
	if err != nil {
		return models.Pipeline{}, fmt.Errorf("failed to parse created_on time %q: %w", p.CreatedOn, err)
	}

	pipeline := models.Pipeline{
		Id:        strings.Trim(p.UUID, "{}"),
		Status:    normalizeBitbucketPipelineStatus(p.State.Name, resultName),
		Ref:       p.Target.RefName,
		Sha:       p.Target.Commit.Hash,
		WebUrl:    p.Links.HTML.Href,
		CreatedAt: createdAt,
	}

	if p.CompletedOn != "" {
		completedAt, err := time.Parse(time.RFC3339Nano, p.CompletedOn)
		if err != nil {
			return models.Pipeline{}, fmt.Errorf("failed to parse completed_on time %q: %w", p.CompletedOn, err)
		}

		pipeline.UpdatedAt = &completedAt
	}

	if p.Trigger.Name != "" {
		source := normalizeBitbucketPipelineTrigger(p.Trigger.Name)
		pipeline.Source = &source
	}

	return pipeline, nil
}

// normalizeBitbucketPipelineStatus maps Bitbucket pipeline state and result to the unified status enum.
func normalizeBitbucketPipelineStatus(stateName, resultName string) models.PipelineStatus {
	switch stateName {
	case "PENDING":
		return models.PipelineStatusPending
	case "IN_PROGRESS":
		return models.PipelineStatusRunning
	case "COMPLETED":
		switch resultName {
		case "SUCCESSFUL":
			return models.PipelineStatusSuccess
		case "FAILED", "ERROR":
			return models.PipelineStatusFailed
		case "STOPPED", "EXPIRED":
			return models.PipelineStatusCancelled
		default:
			return models.PipelineStatusFailed
		}
	case "HALTED", "PAUSED":
		return models.PipelineStatusManual
	default:
		return models.PipelineStatusPending
	}
}

// normalizeBitbucketPipelineTrigger maps Bitbucket trigger names to the unified source enum.
func normalizeBitbucketPipelineTrigger(triggerName string) models.PipelineSource {
	switch triggerName {
	case "PUSH":
		return models.PipelineSourcePush
	case "PULL_REQUEST":
		return models.PipelineSourceMergeRequest
	case "SCHEDULE":
		return models.PipelineSourceSchedule
	case "MANUAL":
		return models.PipelineSourceManual
	case "TRIGGER", "API":
		return models.PipelineSourceTrigger
	default:
		return models.PipelineSourceOther
	}
}

// mapPipelineStatusToBitbucketQuery maps a unified status filter to a Bitbucket query expression.
// NOTE: Some unified statuses map from multiple Bitbucket states (e.g. "failed" normalizes both
// FAILED and ERROR), but the Bitbucket query API does not support OR expressions. The most common
// sub-state is used as the filter value, so filtered results may not include all matching pipelines:
//   - "failed" queries FAILED only (misses ERROR)
//   - "cancelled" queries STOPPED only (misses EXPIRED)
//   - "manual" queries HALTED only (misses PAUSED)
func mapPipelineStatusToBitbucketQuery(status string) string {
	switch status {
	case "pending":
		return `state.name="PENDING"`
	case "running":
		return `state.name="IN_PROGRESS"`
	case "success":
		return `state.result.name="SUCCESSFUL"`
	case "failed":
		return `state.result.name="FAILED"`
	case "cancelled":
		return `state.result.name="STOPPED"`
	case "manual":
		return `state.name="HALTED"`
	default:
		return ""
	}
}

// TriggerPipeline is not supported for Bitbucket.
func (b *BitbucketService) TriggerPipeline(
	_ context.Context,
	_ string,
	_ string,
	_ []models.PipelineVariable,
	_ krci.GitServerSettings,
) (*models.PipelineResponse, error) {
	return nil, fmt.Errorf("trigger pipeline is not supported for Bitbucket")
}
