package bitbucket

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	bitbucketpkg "github.com/KubeRocketCI/gitfusion/pkg/bitbucket"
	"github.com/go-resty/resty/v2"
	"github.com/ktrysmt/go-bitbucket"
)

// defaultBitbucketAPIURL is the base URL for the Bitbucket Cloud REST API.
// NOTE: The go-bitbucket library (used by GetRepository, ListRepositories,
// ListBranches, ListUserOrganizations) also defaults to this URL internally.
// Supporting a configurable API URL (e.g. settings.Url for Bitbucket Data Center)
// would require changes across all Bitbucket methods and the underlying library.
const defaultBitbucketAPIURL = "https://api.bitbucket.org/2.0"

type BitbucketService struct {
	httpClient *resty.Client
}

func NewBitbucketProvider() *BitbucketService {
	return &BitbucketService{
		httpClient: resty.New(),
	}
}

func (b *BitbucketService) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
) (*models.Repository, error) {
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, err
	}

	client := bitbucket.NewBasicAuth(username, password)

	repoOptions := &bitbucket.RepositoryOptions{
		Owner:    owner,
		RepoSlug: repo,
	}

	repository, err := client.Repositories.Repository.Get(repoOptions)
	if err != nil {
		if err.Error() == "404 Not Found" {
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
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, err
	}

	client := bitbucket.NewBasicAuth(username, password)
	repoOptions := &bitbucket.RepositoriesOptions{
		Owner:   account,
		Keyword: listOptions.Name,
	}

	repositories, err := client.Repositories.ListForAccount(repoOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories for account %s: %w", account, err)
	}

	result := make([]models.Repository, 0, len(repositories.Items))
	for _, repo := range repositories.Items {
		result = append(result, *convertBitbucketRepoToRepository(&repo))
	}

	return result, nil
}

// ListUserOrganizations returns workspaces for the authenticated user using go-bitbucket client
func (b *BitbucketService) ListUserOrganizations(
	_ context.Context,
	settings krci.GitServerSettings,
) ([]models.Organization, error) {
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, err
	}

	client := bitbucket.NewBasicAuth(username, password)

	workspaces, err := client.Workspaces.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}

	result := make([]models.Organization, 0, len(workspaces.Workspaces))

	for _, ws := range workspaces.Workspaces {
		org := models.Organization{
			Id:   ws.UUID,
			Name: ws.Name,
		}

		result = append(result, org)
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
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bitbucket token: %w", err)
	}

	client := bitbucket.NewBasicAuth(username, password)
	branchOptions := &bitbucket.RepositoryBranchOptions{
		Owner:    owner,
		RepoSlug: repo,
		Pagelen: 100,
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
	if repo == nil {
		return nil
	}

	// Extract owner username and URL from the generic map fields
	ownerUsername := ""
	if owner, ok := repo.Owner["username"].(string); ok {
		ownerUsername = owner
	}

	repoUrl := ""

	if html, ok := repo.Links["html"].(map[string]interface{}); ok {
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
