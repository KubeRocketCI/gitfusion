package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/go-github/v72/github"
	"golang.org/x/sync/errgroup"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	gfgithub "github.com/KubeRocketCI/gitfusion/pkg/github"
	"github.com/KubeRocketCI/gitfusion/pkg/pointer"
	"github.com/KubeRocketCI/gitfusion/pkg/xiter"
)

const (
	stateOpen   = "open"
	stateClosed = "closed"
	stateMerged = "merged"
)

type GitHubProvider struct {
	httpClient *http.Client
}

func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{}
}

func (g *GitHubProvider) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
) (*models.Repository, error) {
	client := github.NewClient(nil).WithAuthToken(settings.Token)

	repository, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		ghErr := &github.ErrorResponse{}
		if errors.As(err, &ghErr) {
			if ghErr.Response.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("repository %s/%s: %w", owner, repo, gferrors.ErrNotFound)
			}
		}

		return nil, fmt.Errorf("failed to get repository %s/%s: %w", owner, repo, err)
	}

	return convertGitHubRepoToRepository(repository), nil
}

func (g *GitHubProvider) ListRepositories(
	ctx context.Context,
	owner string,
	settings krci.GitServerSettings,
	listOptions models.ListOptions,
) ([]models.Repository, error) {
	client := github.NewClient(nil).WithAuthToken(settings.Token)

	it := filterRepositoriesByName(
		g.listRepositories(ctx, owner, client),
		listOptions,
	)

	result := make([]models.Repository, 0)

	for repo, err := range it {
		if err != nil {
			ghErr := &github.ErrorResponse{}
			if errors.As(err, &ghErr) {
				if ghErr.Response.StatusCode == http.StatusNotFound {
					return nil, fmt.Errorf("organization or user %s: %w", owner, gferrors.ErrNotFound)
				}
			}

			return nil, fmt.Errorf("failed to list repositories for org %s: %w", owner, err)
		}

		result = append(result, *convertGitHubRepoToRepository(repo))
	}

	return result, nil
}

func filterRepositoriesByName(
	scan xiter.Scan[*github.Repository],
	opt models.ListOptions,
) xiter.Scan[*github.Repository] {
	return func(yield func(*github.Repository, error) bool) {
		scan(func(repo *github.Repository, err error) bool {
			if err != nil {
				return yield(nil, err)
			}

			if opt.Name == nil {
				return yield(repo, nil)
			}

			if repo == nil {
				return true
			}

			if strings.Contains(
				strings.ToLower(repo.GetName()),
				strings.ToLower(pointer.ValueOrEmpty(opt.Name)),
			) {
				return yield(repo, nil)
			}

			return true
		})
	}
}

func (g *GitHubProvider) listRepositories(
	ctx context.Context,
	owner string,
	client *github.Client,
) xiter.Scan[*github.Repository] {
	_, _, orgErr := client.Organizations.Get(ctx, owner)
	if orgErr == nil {
		return gfgithub.ScanGitHubList(
			func(opt github.ListOptions) ([]*github.Repository, *github.Response, error) {
				return client.Repositories.ListByOrg(
					ctx,
					owner,
					&github.RepositoryListByOrgOptions{
						ListOptions: opt,
					},
				)
			},
		)
	}

	return gfgithub.ScanGitHubList(
		func(opt github.ListOptions) ([]*github.Repository, *github.Response, error) {
			return client.Repositories.ListByUser(
				ctx,
				owner,
				&github.RepositoryListByUserOptions{
					ListOptions: opt,
				},
			)
		},
	)
}

func convertGitHubRepoToRepository(repo *github.Repository) *models.Repository {
	if repo == nil {
		return nil
	}

	var name, owner, url string
	if repo.Name != nil {
		name = *repo.Name
	}

	if repo.Owner != nil && repo.Owner.Login != nil {
		owner = *repo.Owner.Login
	}

	if repo.HTMLURL != nil {
		url = *repo.HTMLURL
	}

	return &models.Repository{
		DefaultBranch: repo.DefaultBranch,
		Description:   repo.Description,
		Id:            strconv.FormatInt(repo.GetID(), 10),
		Name:          name,
		Owner:         &owner,
		Url:           &url,
		Visibility:    convertVisibility(repo.GetPrivate()),
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

// ListUserOrganizations returns organizations for the authenticated user.
// Also it adds the current user as an organization.
func (g *GitHubProvider) ListUserOrganizations(
	ctx context.Context,
	settings krci.GitServerSettings,
) ([]models.Organization, error) {
	client := github.NewClient(nil).WithAuthToken(settings.Token)
	eg, ctx := errgroup.WithContext(ctx)

	var userOrg *models.Organization
	// Goroutine 1: Get current user
	eg.Go(func() error {
		user, _, err := client.Users.Get(ctx, "")
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}

		userOrg = &models.Organization{
			Id:        strconv.FormatInt(user.GetID(), 10),
			Name:      user.GetLogin(),
			AvatarUrl: user.AvatarURL,
		}

		return nil
	})

	result := make([]models.Organization, 0, 10)

	// Goroutine 2: Get organizations
	eg.Go(func() error {
		it := gfgithub.ScanGitHubList(
			func(opt github.ListOptions) ([]*github.Membership, *github.Response, error) {
				return client.Organizations.ListOrgMemberships(
					ctx,
					&github.ListOrgMembershipsOptions{
						State:       "active",
						ListOptions: opt,
					},
				)
			},
		)

		for membership, err := range it {
			if err != nil {
				return fmt.Errorf("failed to list organizations: %w", err)
			}

			org := membership.Organization
			if org == nil {
				continue
			}

			orgModel := models.Organization{
				Id:        strconv.FormatInt(org.GetID(), 10),
				Name:      org.GetLogin(),
				AvatarUrl: org.AvatarURL,
			}

			result = append(result, orgModel)
		}

		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	if userOrg != nil {
		result = append(result, *userOrg)
	}

	return result, nil
}

// ListBranches implements BranchesProvider for GitHubService.
// Returns all branches for the given repository. Pagination fields in the response reflect the full result.
func (g *GitHubProvider) ListBranches(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
	_ models.ListOptions,
) ([]models.Branch, error) {
	client := github.NewClient(nil).WithAuthToken(settings.Token)

	it := gfgithub.ScanGitHubList(
		func(opt github.ListOptions) ([]*github.Branch, *github.Response, error) {
			branchOpts := &github.BranchListOptions{
				ListOptions: opt,
			}

			return client.Repositories.ListBranches(ctx, owner, repo, branchOpts)
		},
	)

	branches := make([]models.Branch, 0)

	for b, err := range it {
		if err != nil {
			return nil, fmt.Errorf("failed to list branches: %w", err)
		}

		branches = append(branches, models.Branch{
			Name: b.GetName(),
		})
	}

	return branches, nil
}

// ListPullRequests returns pull requests for the given repository with filtering and pagination.
// For "open" and "all" states, GitHub API handles filtering natively.
// For "merged" and "closed" states, post-filtering is required because GitHub API
// only supports state=closed (which includes both merged and truly-closed PRs).
func (g *GitHubProvider) ListPullRequests(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
	opts models.PullRequestListOptions,
) (*models.PullRequestsResponse, error) {
	client := github.NewClient(g.httpClient).WithAuthToken(settings.Token)

	ghState := mapPullRequestStateToGitHub(opts.State)

	if opts.State == stateMerged || opts.State == stateClosed {
		return g.listPullRequestsWithPostFilter(ctx, client, owner, repo, opts, ghState)
	}

	return g.listPullRequestsDirect(ctx, client, owner, repo, opts, ghState)
}

// listPullRequestsDirect handles states that GitHub API supports natively (open, all).
func (g *GitHubProvider) listPullRequestsDirect(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	opts models.PullRequestListOptions,
	ghState string,
) (*models.PullRequestsResponse, error) {
	ghPRs, resp, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
		State: ghState,
		ListOptions: github.ListOptions{
			Page:    opts.Page,
			PerPage: opts.PerPage,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests for %s/%s: %w", owner, repo, err)
	}

	result := make([]models.PullRequest, 0, len(ghPRs))
	for _, pr := range ghPRs {
		result = append(result, convertGitHubPullRequest(pr))
	}

	var total int

	switch {
	case resp.LastPage > 0:
		total = resp.LastPage * opts.PerPage
	case len(ghPRs) < opts.PerPage:
		total = (opts.Page-1)*opts.PerPage + len(result)
	default:
		total = opts.Page * opts.PerPage
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

// ghPostFilterPageSize is the page size used when fetching from GitHub for post-filtered states.
// Using the maximum (100) reduces the number of API round-trips needed.
const ghPostFilterPageSize = 100

// ghPostFilterMaxPages is the maximum number of GitHub API pages to fetch
// when post-filtering (merged/closed). This prevents unbounded API calls
// in pathological cases (e.g., repos with many closed PRs).
const ghPostFilterMaxPages = 50

// listPullRequestsWithPostFilter fetches closed PRs from GitHub page by page,
// applies post-filtering (merged vs truly-closed), and accumulates results
// until PerPage items are collected or all GitHub pages are exhausted.
func (g *GitHubProvider) listPullRequestsWithPostFilter(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	opts models.PullRequestListOptions,
	ghState string,
) (*models.PullRequestsResponse, error) {
	needed := opts.PerPage
	skip := (opts.Page - 1) * opts.PerPage

	result := make([]models.PullRequest, 0, needed)

	ghPage := 1
	pagesQueried := 0

	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("failed to list pull requests for %s/%s: %w", owner, repo, err)
		}

		if pagesQueried >= ghPostFilterMaxPages {
			break
		}

		ghPRs, resp, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
			State: ghState,
			ListOptions: github.ListOptions{
				Page:    ghPage,
				PerPage: ghPostFilterPageSize,
			},
		})

		pagesQueried++

		if err != nil {
			return nil, fmt.Errorf("failed to list pull requests for %s/%s: %w", owner, repo, err)
		}

		for _, pr := range ghPRs {
			if !matchesPullRequestStateFilter(pr, opts.State) {
				continue
			}

			if skip > 0 {
				skip--

				continue
			}

			result = append(result, convertGitHubPullRequest(pr))

			if len(result) >= needed {
				// We filled the page before exhausting GitHub results.
				// Use a lower-bound estimate: at least one more page may exist.
				total := opts.Page*opts.PerPage + 1

				return &models.PullRequestsResponse{
					Data: result,
					Pagination: models.Pagination{
						Total:   total,
						Page:    &opts.Page,
						PerPage: &opts.PerPage,
					},
				}, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}

		ghPage = resp.NextPage
	}

	return &models.PullRequestsResponse{
		Data: result,
		Pagination: models.Pagination{
			Total:   (opts.Page-1)*opts.PerPage + len(result),
			Page:    &opts.Page,
			PerPage: &opts.PerPage,
		},
	}, nil
}

func mapPullRequestStateToGitHub(state string) string {
	switch state {
	case stateMerged, stateClosed:
		return stateClosed
	case stateOpen:
		return stateOpen
	default:
		return "all"
	}
}

// matchesPullRequestStateFilter returns true if the GitHub PR matches the requested state filter.
func matchesPullRequestStateFilter(pr *github.PullRequest, state string) bool {
	switch state {
	case stateMerged:
		return pr.MergedAt != nil
	case stateClosed:
		// "closed" in our model means closed-but-not-merged.
		return pr.MergedAt == nil
	default:
		return true
	}
}

// convertGitHubPullRequest converts a GitHub PR to the internal model.
func convertGitHubPullRequest(pr *github.PullRequest) models.PullRequest {
	var state models.PullRequestState

	switch {
	case pr.MergedAt != nil:
		state = models.PullRequestStateMerged
	case pr.GetState() == stateClosed:
		state = models.PullRequestStateClosed
	default:
		state = models.PullRequestStateOpen
	}

	prModel := models.PullRequest{
		Id:           strconv.FormatInt(pr.GetID(), 10),
		Number:       pr.GetNumber(),
		Title:        pr.GetTitle(),
		State:        state,
		SourceBranch: pr.Head.GetRef(),
		TargetBranch: pr.Base.GetRef(),
		Url:          pr.GetHTMLURL(),
		CreatedAt:    pr.GetCreatedAt().Time,
		UpdatedAt:    pr.GetUpdatedAt().Time,
		Draft:        pr.Draft,
	}

	if pr.GetBody() != "" {
		prModel.Description = pr.Body
	}

	if pr.Head != nil && pr.Head.GetSHA() != "" {
		prModel.CommitSha = pr.Head.SHA
	}

	if pr.User != nil {
		prModel.Author = &models.Owner{
			Id:        strconv.FormatInt(pr.User.GetID(), 10),
			Name:      pr.User.GetLogin(),
			AvatarUrl: pr.User.AvatarURL,
		}
	}

	return prModel
}
